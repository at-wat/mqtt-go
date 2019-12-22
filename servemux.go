package mqtt

// ServeMux is a MQTT message handler multiplexer.
// The idea is very similar to http.ServeMux.
type ServeMux struct {
	handlers []serveMuxHandler
}

type serveMuxHandler struct {
	filter  topicFilter
	handler Handler
}

// HandleFunc registers the handler function for the given pattern.
func (m *ServeMux) HandleFunc(filter string, handler func(*Message)) error {
	return m.Handle(filter, HandlerFunc(handler))
}

// Handle registers the handler for the given pattern.
func (m *ServeMux) Handle(filter string, handler Handler) error {
	f, err := newTopicFilter(filter)
	if err != nil {
		return err
	}
	m.handlers = append(m.handlers, serveMuxHandler{
		filter:  f,
		handler: handler,
	})
	return nil
}

// Serve dispatches the message to the registered handlers.
func (m *ServeMux) Serve(message *Message) {
	for _, h := range m.handlers {
		if h.filter.Match(message.Topic) {
			h.handler.Serve(message)
		}
	}
}
