
function reload_map() {
    let o = document.getElementById('map-svg');
    o.data = null;
    o.data = '/render';
}

function router_change_onchange(e) {
    var o = e.target.options[e.target.selectedIndex];

    let controls = document.getElementById('controls');
    controls.style.display='none';
    
    var pos = document.getElementById('controls-pos');
    pos.value = o.dataset.pos;
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
    xhr.open('POST', `/update/${pos.dataset.id.replace("/","__SLASH__")}`, true);
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
    [
	["controls-left", [-1,0], controls_click],
	["controls-up",   [0,1], controls_click],
	["controls-down", [0,-1], controls_click],
	["controls-right", [1,0], controls_click],
    ].forEach((e) => {
        document.getElementById(e[0]).onclick = (v) => {e[2](e[1], v);}
    });
});
