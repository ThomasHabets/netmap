var layout = "neato";
var map = "main";

function reload_map() {
    let parent = document.getElementById('main-map');
    let oldo = document.getElementById('map-svg');
    let o = document.createElement('object');
    o.onload = (e) => {
	parent.removeChild(oldo);
	o.style = '';
	//o.style.visibility = 'visible';
	o.id = 'map-svg';
    };
    o.type = 'image/svg+xml';
    o.data = `/render?layout=${layout}&map=${map}`;
    //o.style.visibility = 'hidden';
    //o.style="position:absolute;left:100000px"
    o.style = 'position: absolute; opacity: 0; z-index: -10000; height: 10px'
    parent.appendChild(o);
}

function router_change_onchange(e) {
    let o = e.target.options[e.target.selectedIndex];

    let controls = document.getElementById('controls');
    controls.style.display='none';
    
    var pos = document.getElementById('controls-pos');
    if (o.dataset.pos != "") {
      pos.value = o.dataset.pos;
    } else {
      pos.value = "0,0"
    }
    pos.dataset.id = o.dataset.id;
    controls.appendChild(pos);
    
    controls.style.display='block';
}

function controls_pos_keypress(e) {
    if (e.key !== "Enter") {
	return;
    }
    e.preventDefault();

    update_pos();
}

function update_pos() {
    var pos = document.getElementById('controls-pos');
    // TODO: CSRF
    let xy = pos.value.split(',');
    let xhr = new XMLHttpRequest();
    xhr.open('POST', `/update/${map}/${pos.dataset.id.replace("/","__SLASH__")}`, true);
    xhr.setRequestHeader('Content-Type', 'application/json');
    xhr.addEventListener('load', (e) => {
	reload_map();
    });
    xhr.send(JSON.stringify({'x': xy[0], 'y': xy[1]}));
}

function controls_click(parm, ev) {
    let pos = document.getElementById('controls-pos');
    let xy = pos.value.split(',');
    xy = [parseInt(xy[0]) + parm[0], parseInt(xy[1]) + parm[1]]
    pos.value = `${xy[0]},${xy[1]}`;
    update_pos();
}

window.addEventListener('load', (event) => {
    document.getElementById('router-selector').addEventListener('change', router_change_onchange);
    document.getElementById('net-selector').addEventListener('change', router_change_onchange);
    document.getElementById('controls-pos').addEventListener('keypress', controls_pos_keypress);

    document.getElementById('layout-selector').addEventListener('change', (o) => {
	console.log("Changing layout");
	layout = o.target.options[o.target.selectedIndex].dataset.layout;
	reload_map();
    });

    document.getElementById('map-selector').addEventListener('change', (o) => {
	console.log("Changing map");
	map = o.target.options[o.target.selectedIndex].dataset.id;
	reload_map();
    });

    [
	["controls-left", [-1,0], controls_click],
	["controls-up",   [0,1], controls_click],
	["controls-down", [0,-1], controls_click],
	["controls-right", [1,0], controls_click],
    ].forEach((e) => {
        document.getElementById(e[0]).onclick = (v) => {e[2](e[1], v);}
    });
});
