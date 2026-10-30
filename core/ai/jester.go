package ai

// jester plays a uniformly random legal move.
func (b *Bot) jester() int { return b.pick(b.moves) }
