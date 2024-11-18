package connectionManager

const portNum int = 8080
const tcpRetryConnectionTimeoutSeconds int = 1
const tcpWriteTimeoutSeconds int = 2
const tcpEstablishTimeoutSeconds int = 4

// TODO make this reference the Docker config
var ip_list = []string{"server1", "server2"}
