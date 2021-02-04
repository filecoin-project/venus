package ffiwrapper

type Sealer struct {
	sectors  SectorProvider
	stopping chan struct{}
}

func (sb *Sealer) Stop() {
	close(sb.stopping)
}
