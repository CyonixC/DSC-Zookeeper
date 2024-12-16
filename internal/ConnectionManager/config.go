package connectionManager

import "time"

const portNum int = 8080
const tcpRetryConnectionTimeout = 500 * time.Millisecond
const tcpWriteTimeout = 200 * time.Millisecond
const tcpEstablishTimeout = 300 * time.Millisecond
