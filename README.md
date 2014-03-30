nsq_chatroom
============

A Go program which implements a simple chat room based on NSQ.
This application is my first program under Go. It is based on nsq, which is a message distribution system developed by Go. It supports to explicitly bind to one nsqd and communicate with each other through the same 'room', which is corresponding to a topic in nsq. TODO:    Add supporting of nsqlookupd, so that we could connect to more nsqd automatically.

Requirement
============
1. latest NSQ software, e.g. 0.2.27
2. Go 1.2.1

User Guide
============
1. start nsqd
2. start nsq_chatroom, specified with nsqd tcp port
3. CLI:
   r <user>      - user name        
   e <room>      - enter a room     
   l             - leave                   
   s <something> - say something
   h             - help                    
   q             - quit                    

