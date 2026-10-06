package app

import "go-populous2/internal/engine"

type RenderCommand struct {
	X, Y  int
	Actor engine.ActorRef // ActorNone draws the terrain parcel and its overlay.
}

// sourceRenderPlan traverses the eight source rows, then columns. A parcel's
// actor list is drawn oldest to newest by reversing its typed head chain.
func sourceRenderPlan(world *engine.World, cameraX, cameraY int, destination []RenderCommand) []RenderCommand {
	destination = destination[:0]
	if world == nil {
		return destination
	}
	var chain [engine.FollowerCapacity + engine.EffectCapacity + engine.SceneryCapacity + engine.WallCapacity + 2]engine.ActorRef
	for y := cameraY; y < cameraY+viewSize; y++ {
		for x := cameraX; x < cameraX+viewSize; x++ {
			if x < 0 || y < 0 || x >= engine.MapSize || y >= engine.MapSize {
				continue
			}
			destination = append(destination, RenderCommand{X: x, Y: y})
			count := 0
			for ref := world.Actors.Heads[x+y*engine.MapSize]; ref.Kind != engine.ActorNone && count < len(chain); ref = world.Actors.Next(ref) {
				chain[count] = ref
				count++
			}
			for index := count - 1; index >= 0; index-- {
				destination = append(destination, RenderCommand{X: x, Y: y, Actor: chain[index]})
			}
		}
	}
	return destination
}
