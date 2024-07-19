# GRAIN 🌾 WIP

**Go Relay and Information Network**

GRAIN is an open-source Nostr relay implementation written in Go. This project aims to provide a robust and efficient Nostr relay that supports the NIP-01 protocol, focusing on processing user metadata and text notes.

## Features

- **NIP-01 Protocol Support**: GRAIN fully supports the NIP-01 protocol for WebSocket communication.
- **Event Processing**: Handles events of kind 0 (user metadata) and kind 1 (text note).
- **MongoDB Storage**: Utilizes MongoDB to store and manage events efficiently.
- **Scalability**: Built with Go, ensuring high performance and scalability.
- **Open Source**: Licensed under the MIT License, making it free to use and modify.

## Installation

1. **Clone the repository**:

   ```sh
   git clone https://github.com/oceanslim/grain.git
   cd grain
   ```

2. **Build the executable**:

   ```sh
   go build -o grain.exe
   ```

   The `grain.exe` will be placed in a temporary directory within `...\appdata\local\temp\go-build` and subdirectories.

## Usage

To run the GRAIN relay:

```sh
./grain.exe
```

### Configuration 🍃

Configuration options can be set through environment variables or a configuration file. Example configuration:

```yml
server:
  port: 8080
database:
  type: mongodb
  connection_string: mongodb://localhost:27017
  database_name: grain
logging:
  level: info
```

### WebSocket Endpoints

- Connect: / - Clients can connect to this endpoint to start a WebSocket session.
- Publish Event: Send events of kind 0 (user metadata) or kind 1 (text note) to the relay.

### Development

To contribute to GRAIN, follow these steps:

1. Fork the repository.
2. Create a new branch:

```sh
git checkout -b feature-branch
```

3. Make your changes.
4. Commit your changes:

```sh
git commit -m "Description of changes"
```

5. Push to the branch:

```sh
git push origin feature-branch
```

6. Create a Pull Request.

### Contributing

### License

This project is Open Source and licensed under the MIT License. See the [LICENSE](license) file for details.

### Acknowledgments

Special thanks to the Nostr community for their continuous support and contributions.

Feel free to reach out with any questions or issues you encounter while using GRAIN. Happy coding!
