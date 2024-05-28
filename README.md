# gitlab-pusher

This Go application is designed to `git push` to GitLab - after ensuring the repository is created (using `PRIVATE-TOKEN`, interacting with `/api/v4`)

So far, only desinged for pushing new repos, that are dynamically created on disk... that haven't been pushed.

It supports two modes of operation:
1. **Ad-hoc Mode**: Using environment variables and command-line arguments.
2. **Server Mode**: Listening for JSON POST requests to set configuration and execute operations.

## Features

- Creates GitLab Project, based on the `GITLAB_ADDRESS`/`GITLAB_GROUP`/`repo_name` if it doesn't exist already.
- Reads `~/.ssh/config` and uses your keys, so set that up properly for ease of use.
- Push changes to the remote repository.
- Run as a command-line tool or a server.

## Usage

### Prerequisites

- Docker / Golang
- GitLab personal access token

### Environment Variables

Ensure the following environment variables are set:

- `GITLAB_ADDRESS`: Your GitLab instance address, eg. `gitlab.com`  
    - FQDN of the gitlab instance - , currently only HTTPS verified supported. You can surely modify this if you really need HTTP.
- `GITLAB_GROUP`: Your GitLab group, eg. `coolgroup` or `coolgroup/coolsubgroup` in that format...
- `GITLAB_USER_NAME`: Your GitLab Pusher's username, eg. `oxidized`
- `GITLAB_EMAIL`: Your GitLab email, eg. `pusher@gitlab.com`
- `GITLAB_TOKEN`: Your GitLab personal access token, eg. `juftw-8l7sjhdkx43xc1s`
  - Used to connect to GitLab and create the project, if not exists
- `BASE_REPO_DIR`: Base directory for repositories, eg. `/home/oxidized/repos`
  - This directory is the prefixed to `repo_name` 

### Running the Application

#### Ad-hoc Mode

To run the application in ad-hoc mode:

```sh
docker run --rm -it \
  -e GITLAB_ADDRESS=your_gitlab_address \
  -e GITLAB_GROUP=your_gitlab_group \
  -e GITLAB_USER_NAME=your_gitlab_user_name \
  -e GITLAB_EMAIL=your_gitlab_email \
  -e GITLAB_TOKEN=your_gitlab_token \
  -e BASE_REPO_DIR=your_base_repo_dir \
  jseifeddine/gitlab-pusher repo_name
```

or you can run the built binary:

```sh
  GITLAB_ADDRESS=your_gitlab_address \
  GITLAB_GROUP=your_gitlab_group \
  GITLAB_USER_NAME=your_gitlab_user_name \
  GITLAB_EMAIL=your_gitlab_email \
  GITLAB_TOKEN=your_gitlab_token \
  BASE_REPO_DIR=your_base_repo_dir \
  ./gitlab-pusher repo_name
```

Replace the environment variables with your actual values and `repo_name` with the name of your repository.

#### Server Mode

To run the application in server mode:

```sh
docker run --rm -it -p 8080:8080 jseifeddine/gitlab-pusher --listen 8080
```

or simply:

```sh
./gitlab-pusher --listen 8080
```

This will start the server and listen on port 8080 for JSON POST requests.

#### Sending payload via POST Request

To execute a push, via POST request:

```sh
curl -X POST http://localhost:8080/push -H "Content-Type: application/json" -d '{
  "gitlab_address": "your_gitlab_address",
  "gitlab_group": "your_gitlab_group",
  "gitlab_user_name": "your_gitlab_user_name",
  "gitlab_email": "your_gitlab_email",
  "gitlab_token": "your_gitlab_token",
  "base_repo_dir": "your_base_repo_dir",
  "repo_name": "your_repo_name"
}'
```

Replace the JSON values with your actual configuration.


## Building the Docker Image

To build the Docker image:

```sh
docker build -t jseifeddine/gitlab-pusher .
```

## Development

### Prerequisites

- Go 1.18 or later

### Building Locally

To build the application locally:

```sh
go build -o gitlab-pusher main.go
```


## Contributing

Contributions are welcome! Please open an issue or submit a pull request for any changes.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

---

This README should provide a clear overview of the project, its usage, and how to get started with it. If you have any more specific details or requirements, feel free to adjust the content accordingly.
