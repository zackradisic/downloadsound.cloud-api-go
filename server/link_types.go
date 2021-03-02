package server

type linkType int

const (
	linkTypeTrack linkType = iota
	linkTypePlaylist
	linkTypeLikes
)
