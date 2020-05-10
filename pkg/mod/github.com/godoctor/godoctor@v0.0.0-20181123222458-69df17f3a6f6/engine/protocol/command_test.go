// Copyright 2015 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package protocol

import "testing"

func TestAboutValidatePass(t *testing.T) {
	// about requires state > 0 to pass validation
	state := State{State: 1}
	if err := aboutValidate(&state, nil); err != nil {
		t.Fatal("About.Validate: ", err)
	}
}

func TestAboutValidateFail(t *testing.T) {
	// about requires state > 0 to pass validation
	state := State{State: 0}
	if err := aboutValidate(&state, nil); err == nil {
		t.Fatal("About.Validate: should fail with stat < 1")
	} else {
		if err.Error() != "The about command requires a state of non-zero" {
			t.Fatal("About.Validate: error message is not as expected")
		}
	}
}

func TestAboutRunFail(t *testing.T) {
	state := State{State: 0}

	if reply, err := about(&state, nil); err == nil {
		t.Fatal("About.Run: should fail with state < 1")
	} else {
		if reply.Params["message"] != err.Error() {
			t.Fatal("About.Run: error in reply does not match programmatic error")
		}
	}
}

func TestAboutRunPass(t *testing.T) {
	state := State{State: 1, About: "Test About Text"}

	if reply, err := about(&state, nil); err != nil {
		t.Fatal("About.Run: should pass validate with state of 1")
	} else {
		if reply.Params["reply"] != "OK" {
			t.Fatal("About.Run: reply does not indicate command was successful")
		}
		if reply.Params["text"] != "Test About Text" {
			t.Fatal("About.Run: reply has incorrect about text")
		}
	}
}
