Search for Go symbols (types, functions, methods, constants, variables) by name
using the Language Server Protocol (gopls workspace_symbol). Unlike text-based
grep, this tool understands Go semantics — it finds symbol declarations and
reports their kind (Struct, Function, Method, Interface, Constant, etc.) along
with file paths and line numbers.

Use this tool when you need to:
- Find where a type, function, or interface is declared across the workspace
- Discover all symbols matching a name pattern (e.g. "Runner" finds Runner struct, NewRunner func, etc.)
- Get structured symbol information including kind and location

Parameters:
- symbol (required): The Go identifier name to search for (e.g. "Runner", "BuildCatalog", "Provider")
- path (optional): A file or directory path to set the working context for gopls

Output includes file path, line:column, symbol name, and symbol kind for each match.
Returns exit_code 0 on success. Empty output means no matching symbols were found.