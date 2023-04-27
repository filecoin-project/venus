package util

func MergePeers(peerSet1 []string, peerSet2 []string) []string {

	filter := map[string]struct{}{}
	for _, peer := range peerSet1 {
		filter[peer] = struct{}{}
	}

	notInclude := []string{}
	for _, peer := range peerSet2 {
		_, has := filter[peer]
		if has {
			continue
		}
		filter[peer] = struct{}{}
		notInclude = append(notInclude, peer)
	}
	return append(peerSet1, notInclude...)
}
