package main

const historySize = 50

type historyBuffer struct {
	messages [][]byte
}

func (b *historyBuffer) add(data []byte) {
	b.messages = append(b.messages, data)
	if len(b.messages) > historySize {
		b.messages = b.messages[len(b.messages)-historySize:]
	}
}
