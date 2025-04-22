# Directory Tree Generator

A simple Go application that generates a tree-like representation of directory structures. This tool displays the structure in the terminal and saves it to a Markdown file.

## Features

- Recursively traverses directory structures
- Shows directories with trailing slashes (/)
- Sorts entries with directories first, then files alphabetically
- Saves the tree structure to a Markdown file
- Supports custom directory path and output file path via command-line flags

## Installation

### Building from source

```bash
# Clone the repository
git clone https://github.com/ThembinkosiThemba/dirtree.git
cd dirtree

# Build the application
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

- `-path`: Directory path to generate tree from (default: ".")
- `-output`: Output file path (default: "tree_output.md")

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the [License](/LICENSE) file for details.
