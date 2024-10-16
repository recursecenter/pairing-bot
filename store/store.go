package store

import (
	"log"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
)

// fetchAll converts all documents in iter to values of type T. Documents that
// cannot be converted will be skipped.
//
// If the iterator yields an error instead of a document, this returns the
// first such error and stops.
func fetchAll[T any](iter *firestore.DocumentIterator) ([]T, error) {
	var all []T
	defer iter.Stop()

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			return all, nil
		} else if err != nil {
			return nil, err
		}

		var item T
		if err := doc.DataTo(&item); err != nil {
			log.Printf("Skipping %q: %s", doc.Ref.Path, err)
			continue
		}

		all = append(all, item)
	}
}
