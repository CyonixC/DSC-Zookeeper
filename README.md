# ZK Current Tasks
 1. Leader election (1 person)
	- Implement any election algorithm that will allow server cluster machines to elect the leader
	- Should be based on some arbitrary variable for now (in the future it will be based on *zxid*)
	- Should be tolerant to faults
2. Proposals system (1-2 ppl)
	- Implement the necessary structures to handle proposal labelling and syncing.
	- This includes implementing the Request, Proposal, and Commit messages, and the algorithms for handling them.
	- Should be syncing of some arbitrary data for now (incrementing / decrementing some variable, e.g.). In the future, the data being sent will be *znode* data.
3. Znode system (1-2 ppl)
	- Implement the znode system for a single system, with znode files being stored on disk.
	- Should support all API functions in the Client API section of the ZooKeeper paper except `sync()`

# Misc Current Tasks
1. High priority! Set up TCP message handling functions to facilitate message passing (1 person)
	- Should be able to set up and keep track of TCP communication channels between nodes
	- Should output corresponding signal in response to suspected node failure
	- Should be able to handle sending the messages to the correct goroutines on receipt?
2. Docker / Kubernetes setup (1 person?)
	- Do up some documentation for the setup as well
	
# Future Work
1. Leader activation (requires the proposals system)
2. Handle syncing of client requests (requires proposals system and znode system)
3. Incorporate *zxid* into the election (requires proposals system)
4. Handle requests during server election (requires proposals and znode system)
5. Create client-side API that allow clients to call corresponding systems (requires znode system)