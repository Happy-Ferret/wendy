package pastry

import (
	"time"
)

// LeafSet contains the 32 closest Nodes to the current Node, based on the numerical proximity of their NodeIDs.
//
// The LeafSet is divided into Left and Right; the NodeID space is considered to be circular and thus wraps around. Left contains NodeIDs to the left of the current NodeID. Right contains NodeIDs to the right of the current NodeID.
type LeafSet struct {
	left  [16]*Node
	right [16]*Node
	self  *Node
	req   chan *leafSetRequest
	kill  chan bool
}

// leafSetRequest is a request for a specific Node in the LeafSet. The Node field determines the Node being queried against. Should it not be set, the Pos field is used in its stead, with a negative Pos signifying the Nodes with NodeIDs counter-clockwise from the current Node and a positive Pos signifying the Nodes with NodeIDs clockwise from the current Node.
//
//The Mode field is used to determine whether the request is to insert, get, or remove the Node from the LeafSet.
//
//The Left field designates whether the Node is to the left or the right of the current Node in the NodeID space. True means to the left, false means to the right.
//
// Methods that return a leafSetRequest will always do their best to fully populate it, meaning the result can be used to, for example, determine the position of a Node.
type leafSetRequest struct {
	Pos  int
	Left bool
	Node *Node
	resp chan *leafSetRequest
	Mode reqMode
}

// NewLeafSet initialises a new LeafSet along with all its corresponding channels.
func NewLeafSet(self *Node) *LeafSet {
	left := [16]*Node{}
	right := [16]*Node{}
	req := make(chan *leafSetRequest)
	kill := make(chan bool)
	return &LeafSet{
		self:  self,
		left:  left,
		right: right,
		req:   req,
		kill:  kill,
	}
}

// Stop stops a LeafSet from listening for requests.
func (l *LeafSet) Stop() {
	l.kill <- true
}

// Insert inserts a new Node into the LeafSet.
//
// Insert will return a populated leafSetRequest or a TimeoutError. If both returns are nil, Insert was unable to store the Node in the LeafSet, as the Node's ID is the same as the current Node's ID or the Node is nil.
//
// Insert is a concurrency-safe method, and will return a TimeoutError if the leafSetRequest is blocked for more than one second.
func (l *LeafSet) Insert(n *Node) (*leafSetRequest, error) {
	resp := make(chan *leafSetRequest)
	l.req <- &leafSetRequest{Node: n, Mode: mode_set, resp: resp}
	select {
	case r := <-resp:
		return r, nil
	case <-time.After(1 * time.Second):
		return nil, throwTimeout("Node insertion", 1)
	}
	return nil, nil
}

// insert does the actual low-level insertion of a Node. It should *only* be called from the listen method of the LeafSet, to preserve its concurrency-safe property.
func (l *LeafSet) insert(r *leafSetRequest) *leafSetRequest {
	if r.Node == nil {
		return nil
	}
	var pos int
	side := l.self.ID.RelPos(r.Node.ID)
	if side == -1 {
		l.left, pos = r.Node.insertIntoArray(l.left, l.self)
		return &leafSetRequest{Mode: mode_set, Node: r.Node, Pos: pos, Left: true}
	} else if side == 1 {
		l.right, pos = r.Node.insertIntoArray(l.right, l.self)
		return &leafSetRequest{Mode: mode_set, Node: r.Node, Pos: pos, Left: false}
	}
	return nil
}

// insertIntoArray just inserts the given Node into the array of Nodes such that the nodes will be ordered by ID. It's a helper function for inserting a Node into a LeafSet. It returns an array with the Node inserted and the position of the Node in the new array.
func (node *Node) insertIntoArray(array [16]*Node, center *Node) ([16]*Node, int) {
	result := new([16]*Node)
	result_index := 0
	src_index := 0
	inserted := -1
	for result_index < len(result) {
		result[result_index] = array[src_index]
		if inserted >= 0 {
			continue
		}
		if array[src_index] == nil {
			result[result_index] = node
			inserted = result_index
			break
		}
		if array[src_index].ID.Equals(node.ID) {
			inserted = result_index
			continue
		}
		if center.ID.Diff(node.ID).Cmp(center.ID.Diff(result[result_index].ID)) < 0 {
			result[result_index] = node
			inserted = result_index
		} else {
			src_index += 1
		}
		result_index += 1
	}
	return result, inserted
}
