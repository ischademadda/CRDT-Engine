workspace "CRDT Engine Project""Data synchronization engine architecture and demo application" {

    model {
        endUser = person "End User" "Opens a browser and collaboratively edits documents in real time."
        developer = person "Software Developer" "Studies documentation and integrates the Go package crdt-engine into their products."

        demoSystem = softwareSystem "CRDT Demo Platform" "A web platform demonstrating the capabilities of conflict-free data synchronization." {
            tags "Target System"
        }

        // 3. Connections (Who interacts with whom and how)
        endUser -> demoSystem "Edits data and sees changes made by others" 'WebSocket'
        developer -> demoSystem "Analyzes the demo app  lication" "HTTPS"
    }

    views {
        // Configure the display of the context diagram
        systemContext demoSystem "ContextDiagram" "Context diagram for the CRDT platform." {
            include *
            // Top-Bottom layout
            autoLayout tb 
        }

        // Some styles for aesthetics
        styles {
            element "Target System" {
                background #1168bd
                color #ffffff
            }
            element "Person" {
                shape Person
                background #08427b
                color #ffffff
            }
        }
    }
}

