package relay

type Event struct {
	ID        string     `json:"id" bson:"id"`
	PubKey    string     `json:"pubkey" bson:"pubkey"`
	CreatedAt int64      `json:"created_at" bson:"created_at"`
	Kind      int        `json:"kind" bson:"kind"`
	Tags      [][]string `json:"tags" bson:"tags"`
	Content   string     `json:"content" bson:"content"`
	Sig       string     `json:"sig" bson:"sig"`
}