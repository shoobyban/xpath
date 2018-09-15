package xpath

import (
	"encoding/xml"
	"fmt"
	"io"
	"reflect"
	"strings"

	"golang.org/x/net/html"
)

// Node is an item in an xml tree that was compiled to
// be processed via xml paths. A node may represent:
//
//     - An element in the xml document (<body>)
//     - An attribute of an element in the xml document (href="...")
//     - A comment in the xml document (<!--...-->)
//     - A processing instruction in the xml document (<?...?>)
//     - Some Text within the xml document
//
type Node struct {
	Kind NodeKind
	Name xml.Name
	Attr string
	Text []byte

	Nodes []Node
	Pos   int
	End   int

	Up   *Node
	Down []*Node
}

type NodeKind int

const (
	AnyNode NodeKind = iota
	StartNode
	EndNode
	AttrNode
	TextNode
	CommentNode
	ProcInstNode
)

var NodeKinds = []string{
	"Any",
	"Start",
	"End",
	"Attr",
	"Text",
	"Comment",
	"ProcInst",
}

// String returns the string value of node.
//
// The string value of a node is:
//
//     - For element Nodes, the concatenation of all Text Nodes within the element.
//     - For Text Nodes, the Text itself.
//     - For attribute Nodes, the attribute value.
//     - For comment Nodes, the Text within the comment delimiters.
//     - For processing instruction Nodes, the content of the instruction.
//
func (node *Node) String() string {
	if node.Kind == AttrNode {
		return node.Attr
	}
	return string(node.Bytes())
}

// TrimText returns trimmed text node
func (node *Node) TrimText() string {
	return strings.TrimSpace(string(node.Text))
}

// ChildrenMap returns interface{} (normally map[string]interface{}) of children
func (node *Node) ChildrenMap() interface{} {
	_, val := node.getNodeValue()
	return val
}

// ChildrenMap returns interface{} (normally map[string]interface{}) of children
func (node *Node) getNodeValue() (int, interface{}) {
	i := node.Pos
	if node.Kind == AttrNode {
		return i, nil
	}
	if node.Kind == TextNode {
		return i, node.TrimText()
	}
	m := map[string]interface{}{}

	name := ""
	lastKind := node.Kind
	lastname := name
	for i = node.Pos + 1; i < node.End; i++ {
		//		fmt.Println(i, NodeKinds[node.Nodes[i].Kind], node.Nodes[i].Name, " # ", node.Nodes[i].Pos, node.End, string(node.Nodes[i].Text))
		if node.Nodes[i].Kind == StartNode {
			name = node.Nodes[i].Name.Local
			if lastKind == StartNode {
				var nvalue interface{}
				i, nvalue = node.Nodes[i].getNodeValue()
				if v, ok := m[name]; ok {
					if reflect.ValueOf(v).Kind() == reflect.Slice {
						m[name] = append(m[name].([]interface{}), nvalue)
					} else {
						m[name] = append([]interface{}{}, m[name], nvalue)
					}
				} else {
					switch nvalue.(type) {
					case string:
						m[name] = nvalue
					case int:
						m[name] = nvalue
					case float64:
						m[name] = nvalue
					case float32:
						m[name] = nvalue
					default:
						m[name] = append([]interface{}{}, nvalue)
					}
				}
			}
			lastKind = StartNode
		} else if node.Nodes[i].Kind == TextNode {
			if node.Nodes[i+1].Kind == EndNode && node.Nodes[i].TrimText() != "" {
				return i + 1, node.Nodes[i].TrimText()
			}
			if name != "" && node.Nodes[i].TrimText() != "" {
				if v, ok := m[name]; ok {
					if reflect.ValueOf(v).Kind() == reflect.Slice {
						m[name] = append(m[name].([]interface{}), node.Nodes[i].TrimText())
					} else {
						m[name] = append([]interface{}{}, m[name], node.Nodes[i].TrimText())
					}
				} else {
					m[name] = node.Nodes[i].TrimText()
				}
				lastKind = TextNode
			}
		} else if node.Nodes[i].Kind == EndNode {
			name = ""
			if lastKind == EndNode {
				return i, m
			} else if lastKind == TextNode {
				m[lastname] = m[lastname].([]interface{})[0]
			}
			lastKind = EndNode
		} else {
			fmt.Println("Unhandled Node", node)
		}
		lastname = name
	}
	return i, m
}

// Bytes returns the string value of node as a byte slice.
// See Node.String for a description of what the string value of a node is.
func (node *Node) Bytes() []byte {
	if node.Kind == AttrNode {
		return []byte(node.Attr)
	}
	if node.Kind != StartNode {
		return node.Text
	}
	size := 0
	for i := node.Pos; i < node.End; i++ {
		if node.Nodes[i].Kind == TextNode {
			size += len(node.Nodes[i].Text)
		}
	}
	text := make([]byte, 0, size)
	for i := node.Pos; i < node.End; i++ {
		if node.Nodes[i].Kind == TextNode {
			text = append(text, node.Nodes[i].Text...)
		}
	}
	return text
}

// equals returns whether the string value of node is equal to s,
// without allocating memory.
func (node *Node) equals(s string) bool {
	if node.Kind == AttrNode {
		return s == node.Attr
	}
	if node.Kind != StartNode {
		if len(s) != len(node.Text) {
			return false
		}
		for i := range s {
			if s[i] != node.Text[i] {
				return false
			}
		}
		return true
	}
	si := 0
	for i := node.Pos; i < node.End; i++ {
		if node.Nodes[i].Kind == TextNode {
			for _, c := range node.Nodes[i].Text {
				if si > len(s) {
					return false
				}
				if s[si] != c {
					return false
				}
				si++
			}
		}
	}
	return si == len(s)
}

// contains returns whether the string value of node contains s,
// without allocating memory.
func (node *Node) contains(s string) (ok bool) {
	if len(s) == 0 {
		return true
	}
	if node.Kind == AttrNode {
		return strings.Contains(node.Attr, s)
	}
	s0 := s[0]
	for i := node.Pos; i < node.End; i++ {
		if node.Nodes[i].Kind == TextNode {
			text := node.Nodes[i].Text
		NextTry:
			for ci, c := range text {
				if c != s0 {
					continue
				}
				si := 1
				for ci++; ci < len(text) && si < len(s); ci++ {
					if s[si] != text[ci] {
						continue NextTry
					}
					si++
				}
				if si == len(s) {
					return true
				}
				for j := i + 1; j < node.End; j++ {
					if node.Nodes[j].Kind == TextNode {
						for _, c := range node.Nodes[j].Text {
							if s[si] != c {
								continue NextTry
							}
							si++
							if si == len(s) {
								return true
							}
						}
					}
				}
			}
		}
	}
	return false
}

// Parse reads an xml document from r, parses it, and returns its root node.
func Parse(r io.Reader) (*Node, error) {
	return ParseDecoder(xml.NewDecoder(r))
}

// ParseDecoder parses the xml document being decoded by d and returns
// its root node.
func ParseDecoder(d *xml.Decoder) (*Node, error) {
	var nodes []Node
	var text []byte

	// The root node.
	nodes = append(nodes, Node{Kind: StartNode})

	for {
		t, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := t.(type) {
		case xml.EndElement:
			nodes = append(nodes, Node{
				Kind: EndNode,
			})
		case xml.StartElement:
			nodes = append(nodes, Node{
				Kind: StartNode,
				Name: t.Name,
			})
			for _, attr := range t.Attr {
				nodes = append(nodes, Node{
					Kind: AttrNode,
					Name: attr.Name,
					Attr: attr.Value,
				})
			}
		case xml.CharData:
			texti := len(text)
			text = append(text, t...)
			nodes = append(nodes, Node{
				Kind: TextNode,
				Text: text[texti : texti+len(t)],
			})
		case xml.Comment:
			texti := len(text)
			text = append(text, t...)
			nodes = append(nodes, Node{
				Kind: CommentNode,
				Text: text[texti : texti+len(t)],
			})
		case xml.ProcInst:
			texti := len(text)
			text = append(text, t.Inst...)
			nodes = append(nodes, Node{
				Kind: ProcInstNode,
				Name: xml.Name{Local: t.Target},
				Text: text[texti : texti+len(t.Inst)],
			})
		}
	}

	// Close the root node.
	nodes = append(nodes, Node{Kind: EndNode})

	stack := make([]*Node, 0, len(nodes))
	downs := make([]*Node, len(nodes))
	downCount := 0

	for pos := range nodes {

		switch nodes[pos].Kind {

		case StartNode, AttrNode, TextNode, CommentNode, ProcInstNode:
			node := &nodes[pos]
			node.Nodes = nodes
			node.Pos = pos
			if len(stack) > 0 {
				node.Up = stack[len(stack)-1]
			}
			if node.Kind == StartNode {
				stack = append(stack, node)
			} else {
				node.End = pos + 1
			}

		case EndNode:
			node := stack[len(stack)-1]
			node.End = pos
			stack = stack[:len(stack)-1]

			// Compute downs. Doing that here is what enables the
			// use of a slice of a contiguous pre-allocated block.
			node.Down = downs[downCount:downCount]
			for i := node.Pos + 1; i < node.End; i++ {
				if nodes[i].Up == node {
					switch nodes[i].Kind {
					case StartNode, TextNode, CommentNode, ProcInstNode:
						node.Down = append(node.Down, &nodes[i])
						downCount++
					}
				}
			}
			if len(stack) == 0 {
				return node, nil
			}
		}
	}
	return nil, io.EOF
}

// ParseHTML reads an HTML document from r, parses it using a proper HTML
// parser, and returns its root node.
//
// The document will be processed as a properly structured HTML document,
// emulating the behavior of a browser when processing it. This includes
// putting the content inside proper <html> and <body> tags, if the
// provided Text misses them.
func ParseHTML(r io.Reader) (*Node, error) {
	ns, err := html.ParseFragment(r, nil)
	if err != nil {
		return nil, err
	}

	var nodes []Node
	var text []byte

	n := ns[0]

	// The root node.
	nodes = append(nodes, Node{Kind: StartNode})

	for n != nil {
		switch n.Type {
		case html.DocumentNode:
		case html.ElementNode:
			nodes = append(nodes, Node{
				Kind: StartNode,
				Name: xml.Name{Local: n.Data, Space: n.Namespace},
			})
			for _, attr := range n.Attr {
				nodes = append(nodes, Node{
					Kind: AttrNode,
					Name: xml.Name{Local: attr.Key, Space: attr.Namespace},
					Attr: attr.Val,
				})
			}
		case html.TextNode:
			texti := len(text)
			text = append(text, n.Data...)
			nodes = append(nodes, Node{
				Kind: TextNode,
				Text: text[texti : texti+len(n.Data)],
			})
		case html.CommentNode:
			texti := len(text)
			text = append(text, n.Data...)
			nodes = append(nodes, Node{
				Kind: CommentNode,
				Text: text[texti : texti+len(n.Data)],
			})
		}

		if n.FirstChild != nil {
			n = n.FirstChild
			continue
		}

		for n != nil {
			if n.Type == html.ElementNode {
				nodes = append(nodes, Node{Kind: EndNode})
			}
			if n.NextSibling != nil {
				n = n.NextSibling
				break
			}
			n = n.Parent
		}
	}

	// Close the root node.
	nodes = append(nodes, Node{Kind: EndNode})

	stack := make([]*Node, 0, len(nodes))
	downs := make([]*Node, len(nodes))
	downCount := 0

	for pos := range nodes {

		switch nodes[pos].Kind {

		case StartNode, AttrNode, TextNode, CommentNode, ProcInstNode:
			node := &nodes[pos]
			node.Nodes = nodes
			node.Pos = pos
			if len(stack) > 0 {
				node.Up = stack[len(stack)-1]
			}
			if node.Kind == StartNode {
				stack = append(stack, node)
			} else {
				node.End = pos + 1
			}

		case EndNode:
			node := stack[len(stack)-1]
			node.End = pos
			stack = stack[:len(stack)-1]

			// Compute downs. Doing that here is what enables the
			// use of a slice of a contiguous pre-allocated block.
			node.Down = downs[downCount:downCount]
			for i := node.Pos + 1; i < node.End; i++ {
				if nodes[i].Up == node {
					switch nodes[i].Kind {
					case StartNode, TextNode, CommentNode, ProcInstNode:
						node.Down = append(node.Down, &nodes[i])
						downCount++
					}
				}
			}
			if len(stack) == 0 {
				return node, nil
			}
		}
	}
	return nil, io.EOF
}
