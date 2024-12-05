package connectionManager

import "time"

const portNum int = 8080
const tcpRetryConnectionTimeoutSeconds int = 1
const tcpWriteTimeoutSeconds int = 2
const tcpEstablishTimeout = 500 * time.Millisecond
