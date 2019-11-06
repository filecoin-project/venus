package discovery                                                                                                                                 
                                                                                                                                                         
import (                                                                                                                                                
        "context"
        "time"
	
	pstore "github.com/libp2p/go-libp2p-peerstore"
	libp2pdisc "github.com/libp2p/go-libp2p-core/discovery" 
)                                                                                                                                                       
                                                                                                                                                          
// NoopDiscovery satisfies the discovery interface without doing anything                                                                           
type NoopDiscovery struct {}                                                                                                                          
                                                                                                                                                       
// FindPeers returns a dead channel that is always closed                                                                                      
func (sd *NoopDiscovery) FindPeers(ctx context.Context, ns string, opts ...libp2pdisc.Option) (<-chan pstore.PeerInfo, error) {                               
        stupidCh := make(chan pstore.PeerInfo)                                                                                                             
        // the output is immediately closed, discovery requests end immediately                                                                            
        // Callstack:                                                                                                                                      
        // https://github.com/libp2p/go-libp2p-pubsub/blob/55f4ad6eb98b9e617e46641e7078944781abb54c/discovery.go#L157                                    
        // https://github.com/libp2p/go-libp2p-pubsub/blob/55f4ad6eb98b9e617e46641e7078944781abb54c/discovery.go#L287
        // https://github.com/libp2p/go-libp2p-discovery/blob/master/backoffconnector.go#L52
        close(stupidCh)
        return stupidCh, nil
}
                                                                                                                                                           
// Advertise does nothing and returns 1 hour.
func (sd *NoopDiscovery) Advertise(ctx context.Context, ns string, opts ...libp2pdisc.Option) (time.Duration, error) {
        return time.Hour, nil
}           
