package connectionManager

import "time"

const portNum int = 8080
const tcpRetryConnectionTimeout = 500 * time.Millisecond
const tcpWriteTimeout = 2 * time.Second
const tcpEstablishTimeout = 3 * time.Second
