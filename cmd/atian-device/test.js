let ws = new WebSocket("ws://192.168.0.251:1234/api")
ws.onopen = function (){
    console.log('open...');
}
ws.onclose = function (){
    console.log('close');
}
ws.onmessage = function (ev){
    console.log("receive: ",ev.data);
}