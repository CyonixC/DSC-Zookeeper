package proposals

import "time"

const newLeaderResponseTimeout = 3 * time.Second // timeout for servers to respond to newLeader messages
const syncResponseTimeout = 10 * time.Second     // timeout for servers to finish the sync process
