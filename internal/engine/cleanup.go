package engine

// PrepareFollowerDeath performs the source's retained cleanup exactly once.
// Identity and map membership survive until a terminal animation removes the
// actor. Repeated hits or terminal removal cannot charge the statistics twice.
func (w *World) PrepareFollowerDeath(id int) {
	if id <= 0 || id >= FollowerCapacity {
		return
	}
	f := &w.Followers[id]
	if f.State == Inactive || f.CleanupPrepared {
		return
	}
	if f.Owner < 2 {
		player := &w.Players[f.Owner]
		player.Statistics.Metric -= 2
		if player.Leader == id {
			player.Statistics.LeaderLosses++
			player.Statistics.Metric -= 10
			player.Leader = 0
			w.Players[f.Owner].RallyX = int(f.X)
			w.Players[f.Owner].RallyY = int(f.Y)
			w.moveMagnet(int(f.Owner), int(f.X), int(f.Y))
		}
	}
	w.clearHeroLinks(id)
	f.Population = 0
	f.LastDevelopedStage = 0
	f.CleanupPrepared = true
}
