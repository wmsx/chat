package main

type Storage struct {
	*StorageFile
	*PeerStorage
}

func NewStorage(root string) *Storage {
	file := NewStorageFile(root)

	ps := NewPeerStorage(file)

	return &Storage{
		file,
		ps,
	}
}
