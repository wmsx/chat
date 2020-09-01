package main

type ClientSet map[*Client]struct{}

func NewClientSet() ClientSet {
	return make(map[*Client]struct{})
}

func (set ClientSet) Add(client *Client) {
	set[client] = struct{}{}
}

func (set ClientSet) Remove(client *Client) {
	if _, ok := set[client]; !ok {
		return
	}
	delete(set, client)
}

func (set ClientSet) Count() int {
	return len(set)
}

func (set ClientSet) Clone() ClientSet {
	n := make(map[*Client]struct{})
	for k, v := range set {
		n[k] = v
	}
	return n
}