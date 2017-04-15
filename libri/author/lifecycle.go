package author

import "os"

// Close disconnects the author from its librarians and closes the DB.
func (a *Author) Close() error {
	// send stop signal to listener
	a.stop <- struct{}{}

	// disconnect from librarians
	if err :=a.librarians.CloseAll(); err != nil {
		return err
	}

	// close the DB
	a.db.Close()

	return nil
}

// CloseAndRemove closes the author and removes all local state.
func (a *Author) CloseAndRemove() error {
	err := a.Close()
	if err != nil {
		return err
	}
	return os.RemoveAll(a.config.DataDir)
}
