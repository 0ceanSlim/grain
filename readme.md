# GRAIN 🌾

## Go Relay Archetecture for Implementing Nostr

GRAIN is an open-source Nostr relay implementation written in Go. This project aims to provide an efficient and configurable Nostr relay.

## Features

- **Dynamic Event Handling**: Capable of processing a wide range of events, categorized by type and kind, including support for event deletion as per NIP-09.
- **Configurable and Extensible**: Easily customizable through configuration files, with plans for future GUI-based configuration management to streamline server adjustments.
- **Efficient Rate Limiting**: Implements sophisticated rate limiting strategies to manage WebSocket messages, - events, and requests, ensuring fair resource allocation and protection against abuse.
- **Flexible Event Size Management**: Configurable size limits for events, with optional constraints based on - event kind, to maintain performance and prevent oversized data handling.
- **MongoDB Integration 🍃**: Utilizes MongoDB for high-performance storage and management of events, ensuring data integrity and efficient query capabilities.
- **Scalable Architecture**: Built with Go, leveraging its concurrency model to provide high throughput and scalability, suitable for handling large volumes of data and connections.
- **Relay Metadata Support (NIP-11)**: Provides relay metadata in compliance with NIP-11, allowing clients to retrieve server capabilities and administrative contact information.
- **User-Friendly Front-End**: Includes a web interface that displays recent events and supports potential future enhancements like configuration management.
- **Open Source**: Licensed under the MIT License, making it free to use and modify.

## Prerequisites

### MongoDB Server 🍃

GRAIN 🌾 leverages MongoDB for efficient storage and management of events. MongoDB, known for its high performance and scalability, is an ideal choice for handling large volumes of real-time data. GRAIN 🌾 uses MongoDB collections to store events categorized by kind and ensures quick retrieval and manipulation of these events through its robust querying capabilities.

You can get the free Community Server edition of MongoDB from the official MongoDB website:
[MongoDB Community Server](https://www.mongodb.com/try/download/community)  
MongoDB provides extensive documentation and support to help you get started with installation and configuration, ensuring a smooth integration with GRAIN.

## Configuration

Grain will automatically create the configurations and relay metadata files necessary if they do not already exist when you first run the program.

They are created in the root directory of Grain. You can change configurations and relay_metadata here. The relay must be restarted for new configurations to take effect.

## Development

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

Feel free to reach out with any questions or issues you encounter while using GRAIN.

Open Source and made with 💜 by [OceanSlim](https://njump.me/npub1zmc6qyqdfnllhnzzxr5wpepfpnzcf8q6m3jdveflmgruqvd3qa9sjv7f60)
