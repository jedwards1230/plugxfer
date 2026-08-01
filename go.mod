module github.com/jedwards1230/plugxfer

go 1.25.0

// Pin builds to a patched standard library while retaining the Go 1.25
// language floor.
toolchain go1.25.12

require (
	github.com/BurntSushi/toml v1.6.0
	gopkg.in/yaml.v3 v3.0.1
)
