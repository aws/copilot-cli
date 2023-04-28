package delete

type NoOpDeleter struct{}

func (n *NoOpDeleter) CleanResources(app, env, wkld string) error {
	return nil
}
