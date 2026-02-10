# Frontend

The Monitoring Assistant's frontend is a modern web application built with React, TypeScript, and Vite.

## Technologies

*   **Framework:** React
*   **Build Tool:** Vite
*   **Language:** TypeScript
*   **Styling:** Tailwind CSS with `shadcn/ui` and Radix UI components.
*   **Testing:** Vitest and React Testing Library

## Project Structure

The frontend code is located in the `frontend/` directory. The main application logic is within `frontend/src/`.

```
frontend/src/
├── App.tsx          # Main application component
├── main.tsx         # Application entry point
├── styles.css       # Global styles
├── components/      # Reusable React components
│   ├── ui/          # shadcn/ui components
│   └── ...
├── hooks/           # Custom React hooks
├── lib/             # Utility functions
├── services/        # Services for interacting with the backend API
├── test/            # Test files
├── types/           # TypeScript type definitions
└── utils/           # General utility functions
```

### Key Directories

*   **`components/`**: This directory contains all the reusable React components that make up the user interface. It is further subdivided into `ui/` for the `shadcn/ui` components and other custom components.
*   **`hooks/`**: This directory holds custom React hooks that encapsulate and reuse stateful logic.
*   **`services/`**: This is where the logic for communicating with the backend API resides. It includes the SSE streaming client for the chat functionality.
*   **`test/`**: Contains all the tests for the frontend components and logic.
*   **`types/`**: This directory contains TypeScript type definitions, providing type safety throughout the application.

## Development

To start the frontend development server, run:
```bash
make dev
```
This will start the Vite dev server, which provides hot module replacement for a fast development experience.

## Building

To build the frontend for production, run:
```bash
make frontend-build
```
This will create a `dist/` directory with the optimized and bundled assets. For a full production package that embeds the frontend into the Go binary, run `make package`.
