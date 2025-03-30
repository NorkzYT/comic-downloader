
# Contributing to Comic Downloader 

Thank you for your interest in contributing to Comic Downloader! We welcome contributions, issues, and pull requests. Please review the following guidelines to help you get started.

---

## Table of Contents

- [Setting Up Your Environment](#setting-up-your-environment)
- [Reporting Issues](#reporting-issues)
- [Pull Request Process](#pull-request-process)
- [Code Style Guidelines](#code-style-guidelines)
- [Adding GOPATH to Your PATH](#adding-gopath-to-your-path)
- [Code of Conduct](#code-of-conduct)

---

## Setting Up Your Environment

1. **Clone the Repository:**  
   Fork the repository and clone it locally.

2. **Build Instructions:**  
   Follow the build instructions provided in the [README.md](README.md) to set up your local development environment.

3. **Go Environment:**  
   Ensure you have [Go](https://golang.org/doc/install) installed if you plan to work on the core application.

4. **Initialize Dependencies:**  
   To install all necessary dependencies for the project, run:
   ```bash
   go install ./...
   ```
   Alternatively, you can use:
   ```bash
   go mod download
   ```
   This ensures that all required modules are downloaded and installed before you start development.

---

## Reporting Issues

- **Search First:**  
  Before opening a new issue, please check the [existing issues](https://github.com/NorkzYT/comic-downloader/issues) to see if the problem has already been reported.

- **Provide Details:**  
  When reporting an issue, include a clear title, detailed description, and steps to reproduce the problem. Screenshots or logs are appreciated if applicable.

---

## Pull Request Process

1. **Fork & Branch:**  
   Fork the repository and create a new branch for your feature or bug fix.

2. **Commit Messages:**  
   Write clear, concise commit messages that describe your changes.

3. **Testing:**  
   Ensure that your changes pass all tests and do not break existing functionality.

4. **Submit:**  
   Open a pull request against the main branch. Provide a detailed description of your changes and reference any related issues.

---

## Code Style Guidelines

- **Coding Conventions:**  
  Follow standard Go coding conventions and best practices.

- **Comments & Documentation:**  
  Write clear comments to explain your code. Do not remove any existing comments unless necessary.

- **Error Handling:**  
  Implement proper error handling and consider edge cases in your code.

- **Performance:**  
  Optimize for performance while maintaining code readability.

---

## Adding GOPATH to Your PATH

For developers working in a Go development environment, ensure your system's `GOPATH` is included in your `PATH` by running the following command:

```bash
export PATH=$(go env GOPATH)/bin:$PATH
```

---

## Code of Conduct

This project adheres to the [Contributor Covenant Code of Conduct](https://contributor-covenant.org/version/2/1/code_of_conduct/). By contributing, you agree to uphold this code. Please review it to ensure a welcoming and respectful community.

---

Your contributions make Comic Downloader better. If you have any questions or need further assistance, feel free to reach out via GitHub Discussions or open an issue.

Thank you for contributing!
