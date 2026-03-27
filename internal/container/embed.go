package container

import _ "embed"

//go:embed configs/apko.yaml
var apkoConfig []byte //nolint:unused // Used by Build() in build.go

//go:embed configs/prebake.dockerfile
var prebakeDockerfile []byte //nolint:unused // Used by Build() in build.go
