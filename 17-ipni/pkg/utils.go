package ipni

func MakeTopic(topic string) string {
	return "/indexer/ingest/" + topic
}
