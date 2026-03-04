const width = document.getElementById('graph-container').clientWidth;
const height = document.getElementById('graph-container').clientHeight;

const svg = d3.select('#graph-container')
    .append('svg')
    .attr('width', '100%')
    .attr('height', '100%')
    .call(d3.zoom().on("zoom", function (event) {
        g.attr("transform", event.transform);
    }));

const g = svg.append('g');

let nodesMap = new Map();
let linksMap = new Map();
let nodes = [];
let links = [];

const simulation = d3.forceSimulation()
    .force("link", d3.forceLink().id(d => d.id).distance(150))
    .force("charge", d3.forceManyBody().strength(-300))
    .force("center", d3.forceCenter(width / 2, height / 2));

function getColor(node) {
    // Volatility Gradient: 0.0 (blue/cool) to 1.0 (red/hot)
    let score = 0;
    if (node.properties && node.properties.volatility_score !== undefined) {
        score = node.properties.volatility_score;
    } else if (node.Properties && node.Properties.volatility_score !== undefined) {
        score = node.Properties.volatility_score;
    }
    // D3 interpolate cool to hot
    return d3.interpolateTurbo(1 - score); // Turbo goes from 0=blue to 1=red.
}

function isSemanticSeam(nodeId) {
    if (!semanticSeams) return false;
    for (let i = 0; i < semanticSeams.length; i++) {
        if (semanticSeams[i].method_a === nodeId || semanticSeams[i].method_b === nodeId) {
            return true;
        }
    }
    return false;
}

function updateGraph(newNodes, newLinks) {
    // Merge nodes
    newNodes.forEach(n => {
        const id = n.id || n.Id;
        if (!id) return;
        
        if (!nodesMap.has(id)) {
            const props = n.properties || n.Properties || {};
            const normalizedNode = {
                id: id,
                name: n.name || props.name || id,
                properties: props,
                ...n
            };
            nodesMap.set(id, normalizedNode);
            nodes.push(normalizedNode);
        }
    });

    // Merge links
    if (newLinks) {
        newLinks.forEach(l => {
            const linkId = `${l.sourceId}-${l.targetId}-${l.type}`;
            if (!linksMap.has(linkId)) {
                // Ensure link format
                const link = { source: l.sourceId, target: l.targetId, type: l.type };
                linksMap.set(linkId, link);
                links.push(link);
            }
        });
    }

    renderGraph();
}

function renderGraph() {
    // Links
    let link = g.selectAll(".link")
        .data(links, d => `${d.source.id || d.source}-${d.target.id || d.target}-${d.type}`);

    link.exit().remove();

    const linkEnter = link.enter().append("line")
        .attr("class", "link")
        .attr("stroke", "#999")
        .attr("stroke-width", 2)
        .attr("marker-end", "url(#arrowhead)"); // Need arrow marker

    link = linkEnter.merge(link);

    // Nodes
    let node = g.selectAll(".node-group")
        .data(nodes, d => d.id);

    node.exit().remove();

    const nodeEnter = node.enter()
        .append('g')
        .attr('class', 'node-group')
        .call(d3.drag()
            .on("start", dragstarted)
            .on("drag", dragged)
            .on("end", dragended))
        .on("dblclick", handleNodeDoubleClick)
        .on("contextmenu", handleNodeContextMenu);

    nodeEnter.append('circle')
        .attr('class', 'node')
        .attr('r', 20)
        .attr('fill', d => getColor(d))
        .attr('stroke', '#fff')
        .attr('stroke-width', 2);

    nodeEnter.append('text')
        .attr('class', 'node-label')
        .attr('dy', 30)
        .attr('text-anchor', 'middle')
        .text(d => d.name || d.id);

    node = nodeEnter.merge(node);
    
    node.select('circle')
        .attr('class', d => {
            let cls = 'node';
            if (showPinchPoints && pinchPoints.has(d.id)) cls += ' pinch-point';
            if (showSemanticSeams && isSemanticSeam(d.id)) cls += ' semantic-seam';
            return cls;
        });

    // Update simulation
    simulation.nodes(nodes);
    simulation.force("link").links(links);
    simulation.alpha(1).restart();

    simulation.on("tick", () => {
        link
            .attr("x1", d => d.source.x)
            .attr("y1", d => d.source.y)
            .attr("x2", d => d.target.x)
            .attr("y2", d => d.target.y);

        node.attr("transform", d => `translate(${d.x},${d.y})`);
    });
}

function dragstarted(event, d) {
    if (!event.active) simulation.alphaTarget(0.3).restart();
    d.fx = d.x;
    d.fy = d.y;
}

function dragged(event, d) {
    d.fx = event.x;
    d.fy = event.y;
}

function dragended(event, d) {
    if (!event.active) simulation.alphaTarget(0);
    d.fx = null;
    d.fy = null;
}

async function handleNodeDoubleClick(event, d) {
    try {
        const response = await fetch(`/api/query?type=traverse&target=${encodeURIComponent(d.id)}&direction=both&depth=1`);
        const data = await response.json();
        
        if (data && Array.isArray(data)) {
            let fetchedNodes = [];
            let fetchedLinks = [];
            data.forEach(path => {
                if (path.nodes) fetchedNodes.push(...path.nodes);
                if (path.edges) fetchedLinks.push(...path.edges);
            });
            updateGraph(fetchedNodes, fetchedLinks);
        }
    } catch (err) {
        console.error("Failed to fetch neighborhood:", err);
    }
}

let contextNode = null;
const contextMenu = document.getElementById('context-menu');

function handleNodeContextMenu(event, d) {
    event.preventDefault();
    contextNode = d;
    contextMenu.style.display = 'block';
    contextMenu.style.left = `${event.pageX}px`;
    contextMenu.style.top = `${event.pageY}px`;
}

document.addEventListener('click', () => {
    contextMenu.style.display = 'none';
});

document.getElementById('menu-simulate-extraction').addEventListener('click', async () => {
    if (!contextNode) return;
    const target = contextNode.id;
    const impactPanel = document.getElementById('impact-panel');
    const impactContent = document.getElementById('impact-content');
    
    impactPanel.style.display = 'block';
    impactContent.innerHTML = `<p>Simulating extraction for ${target}...</p>`;
    
    try {
        const response = await fetch(`/api/query?type=what-if&target=${encodeURIComponent(target)}`);
        const data = await response.json();
        
        impactContent.innerHTML = ''; // clear

        if (data && (data.severed_edges?.length > 0 || data.orphaned_nodes?.length > 0)) {
            const iWidth = impactPanel.clientWidth - 20;
            const iHeight = 400;

            const isvg = d3.select('#impact-content')
                .append('svg')
                .attr('width', '100%')
                .attr('height', iHeight);
            
            const ig = isvg.append('g').attr('transform', 'translate(0, 20)');

            let yOffset = 0;
            
            ig.append('text').text('Orphaned Nodes:').attr('font-weight', 'bold').attr('y', yOffset);
            yOffset += 20;

            (data.orphaned_nodes || []).forEach(node => {
                ig.append('text')
                    .text(`• ${node.id}`)
                    .attr('y', yOffset)
                    .attr('x', 15)
                    .attr('fill', 'red');
                yOffset += 20;
            });

            yOffset += 10;
            ig.append('text').text('Severed Edges:').attr('font-weight', 'bold').attr('y', yOffset);
            yOffset += 20;

            (data.severed_edges || []).forEach(edge => {
                ig.append('text')
                    .text(`• ${edge.sourceId} → ${edge.targetId}`)
                    .attr('y', yOffset)
                    .attr('x', 15)
                    .attr('fill', 'orange');
                yOffset += 20;
            });
            
            isvg.attr('height', yOffset + 40); // resize to fit
        } else {
            impactContent.innerHTML = `<p>No significant impact found.</p>`;
        }
    } catch (err) {
        impactContent.innerHTML = `<p style="color:red;">Error simulating extraction</p>`;
        console.error("Failed to simulate extraction:", err);
    }
});

// Def arrow marker
svg.append("defs").append("marker")
    .attr("id", "arrowhead")
    .attr("viewBox", "0 -5 10 10")
    .attr("refX", 25)
    .attr("refY", 0)
    .attr("orient", "auto")
    .attr("markerWidth", 6)
    .attr("markerHeight", 6)
    .attr("xoverflow", "visible")
    .append("svg:path")
    .attr("d", "M 0,-5 L 10 ,0 L 0,5")
    .attr("fill", "#999")
    .style("stroke", "none");

document.getElementById('search-button').addEventListener('click', async () => {
    const target = document.getElementById('search-input').value;
    if (!target) return;

    try {
        const response = await fetch(`/api/query?type=search-similar&target=${encodeURIComponent(target)}`);
        const data = await response.json();
        
        if (data && Array.isArray(data)) {
            const fetchedNodes = data.map(item => {
                const node = item.node || item;
                return {
                    id: node.id || node.Id,
                    name: (node.properties && node.properties.name) || (node.Properties && node.Properties.name) || node.name || node.id || node.Id,
                    properties: node.properties || node.Properties,
                    ...item
                };
            });
            // Clear prior state on new search
            nodesMap.clear();
            linksMap.clear();
            nodes = [];
            links = [];
            updateGraph(fetchedNodes, []);
        } else {
            console.error("Invalid response format:", data);
        }
    } catch (err) {
        console.error("Failed to fetch data:", err);
    }
});

async function loadInitialOverview() {
    try {
        const response = await fetch('/api/query?type=overview');
        const data = await response.json();
        
        if (data && data.nodes) {
            const fetchedNodes = data.nodes.map(n => ({
                id: n.id || n.Id,
                name: (n.properties && n.properties.name) || (n.Properties && n.Properties.name) || n.id || n.Id,
                properties: n.properties || n.Properties,
                ...n
            }));
            
            nodesMap.clear();
            linksMap.clear();
            nodes = [];
            links = [];
            updateGraph(fetchedNodes, data.edges || []);
        }
    } catch (err) {
        console.error("Failed to fetch initial overview:", err);
    }
}

// Load it initially
loadInitialOverview();

console.log("GraphDB Visualizer initialized. D3 version:", d3.version);let showPinchPoints = false;
let showSemanticSeams = false;
let pinchPoints = new Set();
let semanticSeams = [];

document.getElementById('pinch-points-button').addEventListener('click', async (e) => {
    showPinchPoints = !showPinchPoints;
    e.target.style.backgroundColor = showPinchPoints ? '#cce5ff' : '';
    
    if (showPinchPoints && pinchPoints.size === 0) {
        try {
            const res = await fetch('/api/query?type=seams');
            const data = await res.json();
            if (Array.isArray(data)) {
                if (data.length === 0) alert("No pinch points found in the database.");
                data.forEach(s => pinchPoints.add(s.seam || s.Seam));
            }
        } catch (err) { console.error(err); }
    }
    renderGraph();
});

document.getElementById('semantic-seams-button').addEventListener('click', async (e) => {
    showSemanticSeams = !showSemanticSeams;
    e.target.style.backgroundColor = showSemanticSeams ? '#cce5ff' : '';
    
    if (showSemanticSeams && semanticSeams.length === 0) {
        try {
            const res = await fetch('/api/query?type=semantic-seams&similarity=0.6');
            const data = await res.json();
            if (Array.isArray(data)) {
                if (data.length === 0) alert("No semantic disconnects found at current threshold.");
                semanticSeams = data;
            }
        } catch (err) { console.error(err); }
    }
    renderGraph();
});
