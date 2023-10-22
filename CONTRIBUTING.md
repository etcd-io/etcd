# How to Contribute to etcd 🚀

🎉 etcd is an exciting project, and we're thrilled that you want to contribute! 🎉

This guide outlines the basics of contributing to etcd, the distributed key-value store. etcd is Apache 2.0 licensed and accepts contributions via GitHub pull requests. Here's a fun and engaging roadmap to get you started on your journey:

## 🕵️‍♂️ Find Something to Work On

1. Explore etcd's [GitHub issue tracker](https://github.com/etcd-io/etcd/issues) to find a task that sparks your interest. There are labels to help you:
   - 🔥 **Good First Issue**: Perfect for beginners to get started.
   - 🌟 **Help Wanted**: Tasks that need your help.
   - 🚀 **Priority/Important**: Work on issues that matter the most.

   If you can't find an issue, don't hesitate to [contact the maintainers](./README.md#Contact) to request more issues.

## 💻 Set Up Your Development Environment

Choose from two options to set up your development environment:

### Option 1: Manual Setup

1. [Clone the repository](https://docs.github.com/en/repositories/creating-and-managing-repositories/cloning-a-repository).
2. Install Go by following the [installation guide](https://go.dev/doc/install). Make sure to check the minimal Go version in the [go.mod file](./go.mod#L3).
3. Install build tools like `make`. On Debian-based distributions, you can run `sudo apt-get install build-essential`.
4. Verify that everything is installed by running `make build`.

### Option 2: Devcontainer Setup

This is a newer, more streamlined environment setup available for etcd versions 3.6 onwards.

- Open the [GitHub Codespaces](https://github.com/features/codespaces) environment by clicking [here](https://github.com/codespaces/new?hide_repo_select=true&ref=main&repo=11225014).
- A ready-to-go development environment will open in your web browser.

**Note**: If you want to use the Devcontainer locally, you'll need Visual Studio Code and Docker.

[File an issue](https://github.com/etcd-io/etcd/issues/new/choose) if you want support for a different development environment.

## 🚀 Implement Your Change

Write code that follows the coding style recommended by the Golang community. You can find details in the [style doc](https://github.com/golang/go/wiki/CodeReviewComments).

Run static analysis (requires [golangci-lint](https://golangci-lint.run/usage/install)):

- `make verify` to check if all tests pass.
- `make verify-*` to run a specific check, e.g., `make verify-bom` to verify the bill-of-materials.json file.
- `make fix` to fix any issues.
- `make fix-*` to fix specific checks.

Don't forget to write unit tests for your changes. New features should also have e2e or integration tests.

## 📝 Commit Your Change

Use this commit message convention:

- Start with the name of the package (e.g., `etcdserver`, `etcdctl`) followed by a colon.
- Describe what your change does.
- Optionally, explain why you made the change in the message body.
- End with `Signed-off-by: FirstName LastName <email@example.com>`. You can add this automatically with `--signoff` in your Git commit command.

Example:

```
etcdserver: Add gRPC interceptor to log incoming requests

To improve debuggability of etcd v3, I added a gRPC interceptor to log
information on incoming requests to the etcd server. The log output includes
remote client info, request content (with value field redacted), request
handling latency, response size, and more. This feature uses the zap logger if available, otherwise, it uses capnslog.

Signed-off-by: FirstName LastName <github@github.com>
```

## 🚀 Create a Pull Request

Follow the [GitHub guide on making a pull request](https://docs.github.com/en/get-started/quickstart/contributing-to-projects#making-a-pull-request). You can also convert your PR to a draft if you're still working on it.

Remember, multiple small PRs are preferred over large ones (more than 500 lines of code).

## 🙌 Get Your Pull Request Reviewed

Before requesting a review, make sure that all GitHub checks have passed. If unrelated tests fail due to flakiness, file an issue to address the problem.

Once all checks are successful, reach out to people involved in the original discussion or [maintainers] for reviews. Depending on the complexity of your PR, it may require approval from 1-2 maintainers before merging.

Thank you for being a part of the etcd community and contributing to this amazing project! 🎉

[maintainers]: https://github.com/etcd-io/etcd/blob/main/MAINTAINERS

Now, let's dive into the etcd world and make some fantastic contributions! 🚀🔑🎨