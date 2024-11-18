# Config Reader
This package is meant to be used to read the json config

## Usage
Import this file in your code:
```go
import (
    // other packages...
    "local/zookeeper/internal/ConfigReader"
)
```

### Usage
```go
// Get a pointer to the Config object
var config_ptr *Config
config_ptr = configReader.GetConfig()


// Get the mode
var mode Mode
mode = configReader.GetMode()

// Get the name
var name string
name = configReader.GetMode()
```

Type of `Config` and `Mode` is as follows:
```go
type Config struct {
	Servers []string `json:"servers"`
	Clients []string `json:"clients"`
}

// Like an enum
const (
	SERVER Mode = iota
	CLIENT
)
```
