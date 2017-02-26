package server

import "os"

// Close handles cleanup involved in closing down the server.
func (l *Librarian) Close() error {
	if err := l.rt.Disconnect(); err != nil {
		return err
	}
	if err := l.rt.Save(l.serverSL); err != nil {
		return err
	}
	l.db.Close()

	return nil
}

// CloseAndRemove cleans up and removes any local state from the server.
func (l *Librarian) CloseAndRemove() error {
	err := l.Close()
	if err != nil {
		return err
	}
	return os.RemoveAll(l.Config.DataDir)
}
