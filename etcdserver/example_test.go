package etcdserver

// func Example_Server() {
// 	flag.Parse() // fills cfg
//
// 	ss, w, err := LoadState(*statedir)
// 	if err != nil {
// 		log.Println("main: unable to load state - %s", err)
// 	}
//
// 	s := Server{
// 		Snapshot: ss,
// 		WalFile:  w,
// 		Config:   cfg,
// 	}
//
// 	go func() {
// 		log.Fatal(http.ListenAndServe(*laddr, s))
// 	}()
// }
