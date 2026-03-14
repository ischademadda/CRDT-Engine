# ADR-0001: Repository structure
*Status*: Accepted

## Context
For most engine compatibility and easy-to-integrate approach repository structure must be basic and follow standart rules of development

## Decision
Implement Standard Go Project Layout with basics Go directories such as:
-  `/cmd` - Main applications for this project.
- `/internal` - Private application and library code
- `/pkg` - Engine core code
- `docs/` - Documentation



