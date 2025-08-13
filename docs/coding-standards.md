### Coding Standards

#### 1. Go Standards (Backend)
* **Formatting:** Use the `gofmt` tool to automatically format the code.
* **Naming:** Variable and function names should be in `camelCase`. Acronyms (like `URL` or `ID`) should be written in uppercase.
* **Comments:** All exported code must have a comment explaining its functionality.

#### 2. C++ Standards (Client)
* **Formatting:** Use a consistent formatting style, such as `clang-format`.
* **Naming:**
    * Classes and Structs: `PascalCase`.
    * Functions and Methods: `camelCase`.
    * Variables: `snake_case`.
* **Best Practices:** Use `std::unique_ptr` and `std::shared_ptr` over raw pointers.

#### 3. JavaScript/React Standards (Portal)
* **Formatting:** Use **ESLint** and a formatter like **Prettier**.
* **Naming:** Variables and functions should be in `camelCase`. React components must be in `PascalCase`.
* **Components:** Use functional components with `hooks` whenever possible.