package resources

func File(name string) ([]byte, bool) {
       data, ok := go_bindata[name]

       if ok == false {
               return nil, false
       }

       return data(), true
}
