package container

import _ "embed"

//go:embed configs/apko.yaml
var apkoConfig []byte

//go:embed configs/prebake.dockerfile
var prebakeDockerfile []byte
