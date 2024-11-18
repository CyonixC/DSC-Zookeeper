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
config_ptr := configReader.GetConfig()
// or
config := *configReader.GetConfig()
```

Structure of config is as follows:
```go
type Config struct {
	Servers []string `json:"servers"`
	Clients []string `json:"clients"`
}
```
