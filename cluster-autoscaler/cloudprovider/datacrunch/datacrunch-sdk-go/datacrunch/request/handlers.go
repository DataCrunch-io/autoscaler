package request

type NamedHandler struct {
	Name string
	Fn   func(*Request)
}

type HandlerList struct {
	list []NamedHandler
}

// Copy handlers list
func (l *HandlerList) copy() HandlerList {
	n := HandlerList{}
	if len(l.list) == 0 {
		return n
	}

	n.list = make([]NamedHandler, len(l.list))
	copy(n.list, l.list)

	return n
}

// Clear handlers list
func (l *HandlerList) Clear() {
	l.list = l.list[:0]
}

// length of handlers list
func (l *HandlerList) Len() int {
	return len(l.list)
}

// push NamedHandler to handlers list
func (l *HandlerList) Push(n NamedHandler) {
	if l.Len() == 0 {
		// initialize handlers list with 5 elements
		l.list = make([]NamedHandler, 0, 5)
	}
	l.list = append(l.list, n)
}

// push handler to handlers list
func (l *HandlerList) PushFunc(f func(*Request)) {
	l.list = append(l.list, NamedHandler{"__anonymous__", f})
}

// push back NamedHandler to handlers list (alias for Push)
func (l *HandlerList) PushBackNamed(n NamedHandler) {
	l.Push(n)
}

// push front function to handlers list
func (l *HandlerList) PushFrontFunc(f func(*Request)) {
	l.PushFront(NamedHandler{"__anonymous__", f})
}

// push front NamedHandler to handlers list
func (l *HandlerList) PushFront(n NamedHandler) {
	if cap(l.list) == len(l.list) {
		// allocate new slice
		l.list = append([]NamedHandler{n}, l.list...)
	} else {
		l.list = append(l.list, NamedHandler{})
		copy(l.list[1:], l.list)
		l.list[0] = n
	}
}

// remove NamedHandler from handlers list
func (l *HandlerList) RemoveByName(name string) {
	// we dont restrict the uniqueness of the name
	// so we need to remove all the handlers with the same name
	for i := 0; i < l.Len(); i++ {
		m := l.list[i]
		if m.Name == name {
			copy(l.list[i:], l.list[i+1:])
			l.list[len(l.list)-1] = NamedHandler{}
			l.list = l.list[:len(l.list)-1]

			// decrease the length of the handlers list
			i--
		}
	}
}

// remove NamedHandler from handlers list
func (l *HandlerList) Remove(n NamedHandler) {
	l.RemoveByName(n.Name)
}

// replace namedhandler by index
func (l *HandlerList) ReplaceByName(name string, r NamedHandler) bool {
	for i := 0; i < l.Len(); i++ {
		m := l.list[i]
		if m.Name == name {
			l.list[i] = r
			return true
		}
	}
	return false
}

// replace namedhandler by index
func (l *HandlerList) Replace(r NamedHandler) bool {
	return l.ReplaceByName(r.Name, r)
}

// update or push back namedhandler
func (l *HandlerList) UpdateOrPushBack(n NamedHandler) {
	if l.Replace(n) {
		return
	}
	l.Push(n)
}

// update or push front namedhandler
func (l *HandlerList) UpdateOrPushFront(n NamedHandler) {
	if l.Replace(n) {
		return
	}
	l.PushFront(n)
}

func (l *HandlerList) Run(r *Request) {
	for _, h := range l.list {
		h.Fn(r)
	}
}

type Handlers struct {
	Validate  HandlerList // Validate parameters
	Build     HandlerList // Build requests
	Unmarshal HandlerList // Unmarshal response
	Complete  HandlerList // Complete Request
}

func (h *Handlers) Copy() Handlers {
	return Handlers{
		Validate:  h.Validate.copy(),
		Build:     h.Build.copy(),
		Unmarshal: h.Unmarshal.copy(),
		Complete:  h.Complete.copy(),
	}
}

func (h *Handlers) Clear() {
	h.Validate.Clear()
	h.Build.Clear()
	h.Unmarshal.Clear()
	h.Complete.Clear()
}

func (h *Handlers) IsEmpty() bool {
	if h.Validate.Len() != 0 {
		return false
	}
	if h.Build.Len() != 0 {
		return false
	}
	if h.Unmarshal.Len() != 0 {
		return false
	}
	if h.Complete.Len() != 0 {
		return false
	}
	return true
}
