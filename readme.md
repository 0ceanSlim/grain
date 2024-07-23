# GRAIN 🌾

**Go Relay Archetecture for Implementing Nostr**

GRAIN is an open-source Nostr relay implementation written in Go. This project aims to provide a robust and efficient Nostr relay that currently supports the NIP-01 of the nostr protocol.

## Features

- **NIP-01 Protocol Support**: GRAIN (nearly)fully supports the NIP-01 for WebSocket communication.
- **Event Processing**: Handles events of kind 0 (user metadata) and kind 1 (text note).
- **MongoDB 🍃**: Utilizes MongoDB to store and manage events efficiently.
- **Scalability**: Built with Go, ensuring high performance and scalability.
- **Open Source**: Licensed under the MIT License, making it free to use and modify.

## Configuration

Configuration options can be set through environment variables or a configuration file.

There is an example config in this repo. Copy the example config to config.yml to get started

```bash
cp config.example.yml config.yml
```

### WebSocket Endpoints

- Connect: / - Clients can connect to this endpoint to start a WebSocket session.
- Publish Event: Send events of kind 0 (user metadata) or kind 1 (text note) to the relay.

### TODO

- Handle more kinds
- Fix Request Query to handle and serve unhandled events by kind
- create whitelist/blacklist functionality
  for:
  - valid nip05 domain
  - pubkey
  - npub
  - kind int
  - kind 1 wordlist
- Rate limit Events.
  - by kind
  - configurable in config.yml

### Development

To contribute to GRAIN, follow these steps:

1. Fork the repository.
2. Make your changes.
3. Commit your changes:

```sh
git commit -m "Description of changes"
```

4. Push to the repo:

```sh
git push
```

5. Create a Pull Request.

### License

This project is Open Source and licensed under the MIT License. See the [LICENSE](license) file for details.

### Acknowledgments

Special thanks to the Nostr community for their continuous support and contributions.

Feel free to reach out with any questions or issues you encounter while using GRAIN. Happy coding!
