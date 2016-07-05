```sequence
Client->GoFD Server: POST /api/v1/server/tasks
GoFD Server-->Client: 200 OK
GoFD Server->GoFD Agent1: POST /api/v1/agent/tasks
GoFD Agent1-->GoFD Server: 200 OK
GoFD Server->GoFD Agent2: POST /api/v1/agent/tasks
GoFD Agent2-->GoFD Server: 200 OK
GoFD Server->GoFD Agent1: POST /api/v1/agent/tasks/start
GoFD Agent1-->GoFD Server: 200 OK
GoFD Server->GoFD Agent2: POST /api/v1/agent/tasks/start
GoFD Agent2-->GoFD Server: 200 OK
GoFD Agent1--GoFD Agent2: p2p translate file
GoFD Agent1->GoFD Server: POST /api/v1/server/tasks/status
GoFD Server-->GoFD Agent1: 200 OK
GoFD Agent2->GoFD Server: POST /api/v1/server/tasks/status
GoFD Server-->GoFD Agent2: 200 OK
 ```