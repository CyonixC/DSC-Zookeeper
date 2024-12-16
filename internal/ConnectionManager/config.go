package connectionManager

import "time"

const portNum int = 8080
const tcpRetryConnectionTimeout = 500 * time.Millisecond
const tcpWriteTimeout = 800 * time.Millisecond
const tcpEstablishTimeout = 1200 * time.Millisecond
