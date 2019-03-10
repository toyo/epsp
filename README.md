# go-epsp
Golang library for EPSP protocol.

EPSP protocol is shown in 
https://p2pquake.github.io/epsp-specifications/epsp-specifications.html

This library implements EPSP protocol.
Need golang 1.9 or later because sync.Map is used. 

If you want to run on P2PQuake network ( https://www.p2pquake.net/ )

    % go get github.com/toyo/epsp/cmd/p2pquake

    % %GOPATH%\bin\p2pquake -d (Win)

    % $GOPATH/bin/p2pquake -d (Unix)

or

    % docker run -Pit toyokun/p2pquake

At the machine which this program runs, you can see EPSP statistics at http://localhost:6980/ or http://[dockerip]:6980/

This main.go doesn't support to send "地震感知情報" (555).
To send this, use peer.WriteExceptFrom() function with from = null.

I welcome your PR.

Thanks.


