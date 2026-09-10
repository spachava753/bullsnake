package runtime

// bufferLease pins a contiguous mutable buffer across Python calls. The frame
// owns cleanup on Go errors; normal and Python-exception paths release directly.
type bufferLease struct {
	data     []byte
	buffer   *byteBuffer
	view     *memoryView
	owner    Value
	released bool
}

// acquireWritableBuffer validates writable contiguous storage before pinning
// the exporter and, when applicable, the source view on the caller's frame.
func acquireWritableBuffer(caller *frame, value Value) (*bufferLease, *Exception) {
	lease := &bufferLease{owner: value}
	if array, ok := value.(*bytearrayValue); ok {
		lease.buffer = array.buffer
	} else if view := viewOf(value); view != nil {
		if view.released {
			return nil, releasedViewError()
		}
		if !view.readonly {
			lease.buffer, lease.view = view.buffer, view
		}
	}
	if lease.buffer == nil {
		return nil, newException("TypeError", "readinto() argument must be read-write bytes-like object, not "+value.TypeName())
	}
	var exception *Exception
	lease.data, exception = binaryData(value)
	if exception != nil {
		return nil, exception
	}
	lease.buffer.pins++
	if lease.view != nil {
		lease.view.pins++
	}
	caller.bufferLeases = append(caller.bufferLeases, lease)
	return lease, nil
}

func (lease *bufferLease) release() {
	if lease.released {
		return
	}
	lease.buffer.pins--
	if lease.view != nil {
		lease.view.pins--
	}
	lease.released = true
	lease.buffer, lease.view, lease.owner, lease.data = nil, nil, nil, nil
}

func releaseBufferLease(caller *frame, lease *bufferLease) {
	lease.release()
	for len(caller.bufferLeases) > 0 && caller.bufferLeases[len(caller.bufferLeases)-1].released {
		last := len(caller.bufferLeases) - 1
		caller.bufferLeases[last] = nil
		caller.bufferLeases = caller.bufferLeases[:last]
	}
}
