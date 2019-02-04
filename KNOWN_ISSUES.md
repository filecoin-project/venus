# Known issues

**WARNING: The Go Filecoin implementation is a *work in progress*.**

`go-filecoin` is development-ready, but **not production ready**.
The majority of development focus to date has been on the block-chain and protocol building blocks. 
There are a number of security and scaling issues that we are yet to shake out.

This implementation is evolving from a prototype to a reference implementation. 
You should carefully consider the work-in-progress state of this effort when deciding whether and how to run a filecoin node.

Specific known security shortcomings we have yet to resolve include, but are not limited to:

- Bootstrapping is not secure, e.g. there is nothing to prevent a eclipse attack.
- There is no mechanism to mitigate spamming by bad players in the network.
- Keys in the wallet are not encrypted.
- The proofs implementation is incomplete.
- Protocol implementations are incomplete, including
    - incomplete consensus rules (blocks not signed, tickets not properly checked, no finality),
    - no slashing for bad behavior,
    - no penalties for miners omitting a proof, mining power isn't verified.
- The HTTP RPC endpoints are not secured. 
Anyone who can open a connection to the RPC API port of the node can issue requests, including administrative actions and transfers of value.
- Inputs are not sanitised; bad input can likely panic the node.
- Content checking is not strictly enforced.
- Client data is not encrypted by default.
- Some caches grow without bound, presenting a DOS vector.
- Faucet rate limiting is primitive and not difficult to subvert.
- Dashboard statistics are easy to pollute.

When performing new work, contributors should observe stronger security-consciousness, 
including being sure that:

- DOS vectors are identified and defended (e.g. caches cannot grow infinitely);
- unexpected or malformed inputs cannot cause a panic or the process to exit;
- information, including error messages, cannot not leak, for example in a network message;
- content reference by content identifier is validated as having that identifier.

Having said all this, **we invite and welcome [contribution of issues](https://github.com/filecoin-project/go-filecoin/issues)** 
for specific problems in the code. 
If you notice a security issue please label it with the `i-security` label. 
If you notice a scaling issue please label it with the `i-scaling` label.