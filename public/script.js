let socket = new WebSocket("ws://localhost:3000/me/ws/connect")
let Message;

run().catch((err) => console.log(err))

async function run() {
  const root = await protobuf.load("/protobuf/user_message.proto")
  Message = root.lookupType("messagepackage.MyMessage")
}

socket.binaryType = "arraybuffer"

socket.onopen = () => {

  socket.onmessage = (message) => {
    const data = new Uint8Array(message.data)
    const decodedMessage = Message.decode(data)

    const chat = document.getElementById("chat_room")
    const div = document.createElement("div")
    div.innerText = `${decodedMessage.username} - ${decodedMessage.text}`

    chat.appendChild(div)
  }
}

