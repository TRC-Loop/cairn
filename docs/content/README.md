# Markdown features available

Quick reference for all the Markdown / Material features available in this docs site. Delete this file or move it once you no longer need the cheatsheet.

## Callouts (admonitions)

!!! note
    A note callout. Use for general info.

!!! tip "Custom title"
    A tip callout with a custom title.

!!! warning
    A warning callout.

!!! danger
    A danger callout for serious issues.

!!! info
    An info callout.

!!! success
    A success callout.

!!! example
    An example callout — useful for code samples in context.

??? note "Collapsible callout"
    Click to expand. Useful for long-but-optional content.

## Code fences

```go
func main() {
    fmt.Println("Hello, Cairn")
}
```

```bash
$ cairn backup -o cairn-backup.db
```

With line numbers and highlighting:

```go linenums="1" hl_lines="2 3"
func handler(w http.ResponseWriter, r *http.Request) {
    user := userFromContext(r.Context())
    log.Info("got request", "user", user.Username)
}
```

Inline `code` works too.

## Emojis

:material-check-circle: :material-alert: :material-database: :smile: :rocket:

Material icons (`:material-X:`) and standard emojis (`:smile:`) both work. Catalog: https://squidfunk.github.io/mkdocs-material/reference/icons-emojis/.

## Tables

| Feature       | Status            | Notes                  |
|---------------|-------------------|------------------------|
| Monitors      | :material-check:  | All 10 types supported |
| Incidents     | :material-check:  | Auto + manual          |
| Status pages  | :material-check:  | Public + embed widget  |

## Tabs

=== "macOS"
    ```sh
    brew install cairn
    ```

=== "Linux"
    ```sh
    curl -LO https://github.com/TRC-Loop/cairn/releases/latest/download/cairn-linux-amd64
    ```

=== "Docker"
    ```sh
    docker run -p 8080:8080 ghcr.io/trc-loop/cairn:latest
    ```

## Task lists

- [x] Set up docs framework
- [ ] Write the actual content
- [ ] Deploy to cairn.arne.sh

## Footnotes

Cairn uses SQLite by default[^sqlite].

[^sqlite]: PostgreSQL support is planned for v1.5.

## Cross-references

Internal links use relative paths: `[text](operations/backup.md)`.

## attr_list — buttons, classes, IDs

[Download Cairn](https://github.com/TRC-Loop/cairn/releases){ .md-button .md-button--primary }

## Full reference

https://squidfunk.github.io/mkdocs-material/reference/
