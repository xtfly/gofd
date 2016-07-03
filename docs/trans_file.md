```sequence
participant Agent1
participant Agent2
participant Agent3
Agent2->Agent1: TCP Connect(with auth header)
Agent1->Agent2: 0xFF
Agent2->Agent1: send BITFIELD
Agent1->Agent2: send BITFIELD
Agent3->Agent2: TCP Connect(with auth header)
Agent2->Agent3: 0xFF
Agent3->Agent2: send BITFIELD
Agent2->Agent3: send BITFIELD
Agent2->Agent1: send REQUEST
Agent1->Agent2: send PIECE
Agent2->Agent3: send HAVE
Agent3->Agent2: send REQUEST
Agent2->Agent3: send PIECE
 ```