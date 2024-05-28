package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"github.com/mitchellh/go-homedir"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/kevinburke/ssh_config"
	stdssh "golang.org/x/crypto/ssh"
)

type Namespace struct {
	ID       int    `json:"id"`
	FullPath string `json:"full_path"`
}

type Config struct {
	GitlabAddress  string `json:"gitlab_address"`
	GitlabGroup    string `json:"gitlab_group"`
	GitlabUserName string `json:"gitlab_user_name"`
	GitlabEmail    string `json:"gitlab_email"`
	GitlabToken    string `json:"gitlab_token"`
	BaseRepoDir    string `json:"base_repo_dir"`
	RepoName       string `json:"repo_name"`
}

var cfg Config

func Error(format string) {
	log.Fatalf("\x1b[31;1m"+format+"\x1b[0m\n")
	os.Exit(1)
}

func Info(format string) {
	log.Printf("\x1b[34;1m"+format+"\x1b[0m\n")
}

func Warning(format string) {
	log.Printf("\x1b[36;1m"+format+"\x1b[0m\n")
}

func getEnvVar(key string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		return ""
	}
	return value
}

func handlePostRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var conf Config
	err := json.NewDecoder(r.Body).Decode(&conf)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	cfg = conf
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Configuration received successfully")
	
	executeGitOperations()
}

func startServer(port string) {
	http.HandleFunc("/push", handlePostRequest)
	log.Printf("Starting server on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func executeGitOperations() {
	home, err := homedir.Dir()
	if err != nil {
		Error(fmt.Sprintf("Failed to get home directory: %v", err))
	}

	missing := ""

	if cfg.GitlabAddress == "" {
		missing += "GITLAB_ADDRESS "
	}
	if cfg.GitlabGroup == "" {
		missing += "GITLAB_GROUP "
	}
	if cfg.GitlabUserName == "" {
		missing += "GITLAB_USER_NAME "
	}
	if cfg.GitlabEmail == "" {
		missing += "GITLAB_EMAIL "
	}
	if cfg.GitlabToken == "" {
		missing += "GITLAB_TOKEN "
	}
	if cfg.BaseRepoDir == "" {
		missing += "BASE_REPO_DIR "
	}

	if missing != "" {
		Error(fmt.Sprintf("Missing var(s): %s", missing))
		os.Exit(1)
	}

	authorizedGroupURL := fmt.Sprintf("https://%s:%s@%s/%s", cfg.GitlabUserName, cfg.GitlabToken, cfg.GitlabAddress, cfg.GitlabGroup)

	normalizeRepoName := func(repoName string) string {
		if !strings.HasSuffix(repoName, ".git") {
			repoName += ".git"
		}
		return repoName
	}

	determineSSHPort := func() string {
		sshPort := os.Getenv("GITLAB_SSH_PORT")
		if sshPort != "" {
			return string(sshPort)
		} else {
			return string(ssh_config.Get(cfg.GitlabAddress, "Port"))
		}
	}

	sshPort := determineSSHPort()

	cfg.RepoName = normalizeRepoName(cfg.RepoName)
	sshRepoURL := fmt.Sprintf("ssh://git@%s:%s/%s/%s", cfg.GitlabAddress, sshPort, cfg.GitlabGroup, cfg.RepoName)
	publicRepoURL := fmt.Sprintf("https://%s/%s/%s", cfg.GitlabAddress, cfg.GitlabGroup, cfg.RepoName)

	createGitlabRepo := func(repoName string) {
		namespacesURL := fmt.Sprintf("https://%s/api/v4/namespaces", cfg.GitlabAddress)
		req, err := http.NewRequest("GET", namespacesURL, nil)
		if err != nil {
			Error(fmt.Sprintf("Error creating request: %v", err))
		}
		req.Header.Set("PRIVATE-TOKEN", cfg.GitlabToken)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			Error(fmt.Sprintf("Error making request: %v", err))
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			Error(fmt.Sprintf("Error reading response body: %v", err))
		}

		var namespaces []Namespace
		if err := json.Unmarshal(body, &namespaces); err != nil {
			Error(fmt.Sprintf("Error unmarshalling response body: %v", err))
		}

		var namespaceID int
		for _, namespace := range namespaces {
			if namespace.FullPath == cfg.GitlabGroup {
				namespaceID = namespace.ID
				break
			}
		}

		Info(fmt.Sprintf("Attempting to create Project %s", publicRepoURL))

		projectURL := fmt.Sprintf("https://%s/api/v4/projects", cfg.GitlabAddress)
		projectData := fmt.Sprintf("name=%s&visibility=private&namespace_id=%d", repoName, namespaceID)

		req, err = http.NewRequest("POST", projectURL, bytes.NewBufferString(projectData))
		if err != nil {
			Error(fmt.Sprintf("Error creating request: %v", err))
		}
		req.Header.Set("PRIVATE-TOKEN", cfg.GitlabToken)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err = client.Do(req)
		if err != nil {
			Error(fmt.Sprintf("Error making request: %v", err))
		}
		defer resp.Body.Close()

		_, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			Error(fmt.Sprintf("Error reading response body: %v", err))
		}
	}

	Info(fmt.Sprintf("Checking if Project \"%s\" exists", publicRepoURL))

	repoPath := filepath.Join(cfg.BaseRepoDir, fmt.Sprintf("%s", cfg.RepoName))
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		Info(fmt.Sprintf("Repository directory %s does not exist in %s", cfg.RepoName, cfg.BaseRepoDir))
	}

	remoteRepoExists := func(url string) bool {
		req, err := http.NewRequest("HEAD", url, nil)
		if err != nil {
			Info(fmt.Sprintf("Error creating HTTP request: %v", err))
			return false
		}

		req.Header.Set("PRIVATE-TOKEN", cfg.GitlabToken)

		client := http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			Info(fmt.Sprintf("Error sending HTTP request: %v", err))
			return false
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			return true
		}
		return false
	}

	if remoteRepoExists(fmt.Sprintf("%s/%s", authorizedGroupURL, cfg.RepoName)) {
		Info(fmt.Sprintf("Project %s does exist", publicRepoURL))
	} else {
		Info(fmt.Sprintf("Project %s does not exist, creating...", publicRepoURL))
		createGitlabRepo(cfg.RepoName)
	}

	r, err := git.PlainOpen(repoPath)
	if err != nil {
		Error(fmt.Sprintf("Failed to open repository: %s, Reason: %s", repoPath, err))
	}

	gitConfig, err := r.Config()
	if err != nil {
		Error(fmt.Sprintf("Failed to get cfg: %v", err))
	}
	gitConfig.User.Name = cfg.GitlabUserName
	gitConfig.User.Email = cfg.GitlabEmail
	err = r.SetConfig(gitConfig)
	if err != nil {
		Error(fmt.Sprintf("Failed to set cfg: %v", err))
	}

	r.DeleteRemote("origin")
	_, err = r.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{sshRepoURL},
	})

	if err != nil {
		Error(fmt.Sprintf("Failed to add remote: %v", err))
	}

	var defaultSSHKeyPaths = []string{
		fmt.Sprintf("%s/.ssh/id_ed25519", home),
		fmt.Sprintf("%s/.ssh/id_rsa", home),
	}

	getSSHKeyPath := func() string {
		privateKeyFile := os.Getenv("SSH_KEY_PATH")
		if privateKeyFile != "" {
			return privateKeyFile
		}
		for _, path := range defaultSSHKeyPaths {
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
		return ""
	}

	privateKeyFile := getSSHKeyPath()
	if privateKeyFile == "" {
		Error(fmt.Sprintf("No SSH key found in default paths"))
	}

	Info(fmt.Sprintf("Using identity: %s", privateKeyFile))

	privateKey, err := ioutil.ReadFile(privateKeyFile)
	if err != nil {
		Error(fmt.Sprintf("Failed to read SSH private key: %v", err))
	}

	signer, err := stdssh.ParsePrivateKey(privateKey)
	if err != nil {
		Error(fmt.Sprintf("Failed to parse SSH private key: %v", err))
	}

	auth := &ssh.PublicKeys{
		User:   "git",
		Signer: signer,
	}

	pushResult := r.Push(&git.PushOptions{
		Auth:           auth,
		Progress:       os.Stdout,
	})

	if pushResult == git.NoErrAlreadyUpToDate {
		Info("No changes to push")
	} else if pushResult != nil {
		Error(fmt.Sprintf("Failed to push to: %v, error: %v", sshRepoURL, pushResult))
	} else {
		Info("Push successful")
	}
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--listen" {
		if len(os.Args) != 3 {
			fmt.Println("Usage: --listen <port>")
			return
		}
		startServer(os.Args[2])
		return
	}

	if len(os.Args) != 2 {
		fmt.Println("Usage: <repo_name>")
		return
	}

	cfg.GitlabAddress = getEnvVar("GITLAB_ADDRESS")
	cfg.GitlabGroup = getEnvVar("GITLAB_GROUP")
	cfg.GitlabUserName = getEnvVar("GITLAB_USER_NAME")
	cfg.GitlabEmail = getEnvVar("GITLAB_EMAIL")
	cfg.GitlabToken = getEnvVar("GITLAB_TOKEN")
	cfg.BaseRepoDir = getEnvVar("BASE_REPO_DIR")
	cfg.RepoName = os.Args[1]

	executeGitOperations()
}
