package ws

import "testing"

func TestHubAddAndRemoveChatClient(t *testing.T) {
	hub := NewHub()

	hub.AddChatClient(1, nil)
	if len(hub.chatRooms) != 1 {
		t.Fatalf("expected chat room to be created")
	}

	hub.RemoveChatClient(1, nil)
	if len(hub.chatRooms) != 0 {
		t.Fatalf("expected chat room to be removed")
	}
}

func TestHubAddAndRemoveGroupClient(t *testing.T) {
	hub := NewHub()

	hub.AddGroupClient(2, nil)
	if len(hub.groupRooms) != 1 {
		t.Fatalf("expected group room to be created")
	}

	hub.RemoveGroupClient(2, nil)
	if len(hub.groupRooms) != 0 {
		t.Fatalf("expected group room to be removed")
	}
}
