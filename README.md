## dirtree

This program is a Go code analyzer that examines a Go repository and generates a detailed structural analysis of the codebase in markdown format. Here's a breakdown of its functionality:

- It parses Go source files to create a hierarchical representation of packages, functions, methods, structs, and interfaces.
- It analyzes how functions and methods call each other, creating a visual representation of these relationships using Mermaid diagrams.
- It identifies and ranks the most frequently called functions in the codebase.
- It easily gives you an idea of the code structure and statistics making it easier for you to understand it.

## Installation

```bash
git install https://github.com/ThembinkosiThemba/dirtree.git
```

or building the project from source

```bash
git clone https://github.com/ThembinkosiThemba/dirtree.git
cd dirtree
go build -o dirtree

# Optional: Install to your $GOPATH/bin
go install
```

## Usage

```bash
# Basic usage (current directory)
./dirtree

# Specify a directory path
./dirtree -path=/path/to/directory

# Specify an output file
./dirtree -output=structure.md

# Specify both path and output
./dirtree -path=/path/to/directory -output=structure.md
```

### Command-line Options

| Flag       | Description                          | Default                 |
| ---------- | ------------------------------------ | ----------------------- |
| `-path`    | Path to the Go repository to analyze | Current directory (`.`) |
| `-output`  | Output file path                     | `code_structure.md`     |
| `-verbose` | Enable verbose logging               | `false`                 |

### Sample Output

> You can check a sample report [here](./code_structure.md)

The output is a markdown file with sections for:

- Project statistics (files, functions, methods, etc.)
- Module information
- Entry points (main packages)
- Directory structure
- Code structure (packages, functions, types)
- Function call graph (visualized with Mermaid)
- Most called functions table

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the [License](/LICENSE) file for details.
