# Logger
This package contains an implementation of a level-based logger, based on the `slog` package.

Use this to log instead of `fmt.Println` for easier control!

## Usage
Import this file in your code:
```go
import (
    // other packages...
    "local/zookeeper/internal/logger"
)
```

For quick logging, this provides 5 functions:

```go
logger.Debug(fmt.Sprint("This is level ", 1))
logger.Info(fmt.Sprint("This is level ", 2))
logger.Warn(fmt.Sprint("This is level ", 3))
logger.Error(fmt.Sprint("This is level ", 4))
logger.Fatal(fmt.Sprint("This is level ", 5))  // equivalent to Error with os.exit(1) afterwards
```

To customise the logger used, call the `InitLogger` function. You can create your own logger, or use the handler included in this folder.

```go
handler := logger.NewPlainTextHandler(slog.LevelWarn)
lg := slog.New(handler)
logger.InitLogger(lg)
```