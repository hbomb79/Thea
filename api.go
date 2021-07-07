package main

import "net/http"

func setupApiRoutes(router *Router) {
	// -- BEGIN API v0 routes -- //
	router.CreateRoute("v0/queue/", apiQueueIndex).Methods("GET")
	router.CreateRoute("v0/queue/{item_id}", apiQueueGet).Methods("GET")
	router.CreateRoute("v0/queue/{item_id}", apiQueueUpdate).Methods("PUSH")

	router.CreateRoute("v0/troubles/", apiTroubleIndex).Methods("GET")
	router.CreateRoute("v0/troubles/{trouble_id}", apiTroubleGet).Methods("GET")
	router.CreateRoute("v0/troubles/{trouble_id}", apiTroubleUpdate).Methods("PUSH")
	// -- ENDOF API v0 routes -- //
}

// apiQueueIndex returns the current processor queue
func apiQueueIndex(w http.ResponseWriter, r *http.Request) {}

// apiQueueGet returns full details for a queue item at the index {item_id} inside the queue
func apiQueueGet(w http.ResponseWriter, r *http.Request) {}

// apiQueueUpdate pushes an update to the processor dictating the new
// positioning of a certain queue item. This allows the user to
// reorder the queue by sending an item to the top of the
// queue, therefore priorisiting it - similar to the Steam library
func apiQueueUpdate(w http.ResponseWriter, r *http.Request) {}

// apiTroubleIndex returns the list of current trouble states
// in the processor
func apiTroubleIndex(w http.ResponseWriter, r *http.Request) {}

// apiTroubleGet returns more details regarding a particular trouble
// state on the processor
func apiTroubleGet(w http.ResponseWriter, r *http.Request) {}

// apiTroubleUpdate allows a push method to provide the details
// needed by the trouble state in order to rectify it - these
// details are unique to each trouble state and are accessible via
// the apiTroubleIndex or apiTroubleGet methods
func apiTroubleUpdate(w http.ResponseWriter, r *http.Request) {}
