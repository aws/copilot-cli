---
title: "Installation"
linkTitle: "Installation"
weight: 3
---
Installing the Copilot CLI currently requires you to download our binary from the GitHub releases page manually. 
In the future, we'll distribute the binary through homebrew and other binaries as well.

In the meantime, to install, copy and paste the command into your terminal.

```bash
# MacOS
curl -Lo /usr/local/bin/ecs-preview https://github.com/aws/amazon-ecs-cli-v2/releases/download/v0.0.9/ecs-preview-darwin-v0.0.9 && \
chmod +x /usr/local/bin/ecs-preview && \
ecs-preview --help
```
```bash
# Linux
curl -Lo /usr/local/bin/ecs-preview https://github.com/aws/amazon-ecs-cli-v2/releases/download/v0.0.9/ecs-preview-linux-v0.0.9 && \
chmod +x /usr/local/bin/ecs-preview && \
ecs-preview --help
```
