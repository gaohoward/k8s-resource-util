# Kubernetes Resources Utility

This is a tool that help users to manage Kubernetes resources.
User can use this tool to create resources. The resources are grouped into collections that can be deployed together with some common purposes. For example a collection of resources usually can be deployed in to one namespace.

![app](doc/app.png)

## Features

- Generating template contents based on kinds
- Cr editing
- Repository save and load
- Deployment management
- Showing Schema for a kind
- in app logging

## Getting Started

1. Clone the repository:
  ```bash
  git clone https://github.com/gaohoward/k8s-resource-util.git
  ```
2. Navigate to the project directory:
  ```bash
  cd k8s-resources-util
  ```
3. Building (Require Go Lang)
  ```
  go build
  ```
4. Starting
  ```
  ./resutil
  ```

## Note

If you get the following build error:
```
Package 'xkbcommon-x11' not found
```
You need to install the missing dependency xkbcommon-x11 (Fedora for example):
```
sudo dnf install libxkbcommon libxkbcommon-devel libxkbcommon-utils
```

## Contributing

Contributions are welcome! Please submit a pull request or open an issue for any suggestions or improvements.

## License

This project is licensed under the [MIT License](LICENSE).
