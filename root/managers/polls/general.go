package polls

func (man *PollManager) Close() error {
	return man.queue.Close()
}
func (man *PollManager) Len() int {
	return man.queue.Len()
}
