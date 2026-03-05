
Feedback on the UI

When I first navigate in, it seems I'm in a random place zoomed in,  if I zoom out there's just 1 big sphere of nodes, no connections between them -- browse for yourself and see.  When I first start, I want to see the top level domains, in this application I believe there are 4 or 5... then double clicking on one of those domains should show me the features, sub features, etc... (this is the semantic layer), for the file layer... let's come up with another top level view, I'm open to ideas... but we should start at the semantic layer and that should be the focus for the starting point.
When I click on semantic layer or physical layer it seems these are still not wired up? 
when I search for something, I can't tell that the search completed, i.e. search for "tolls" and the screen doesn't update, I have to zoom out/in to find it.                                   
   when I double click a node, it doesn't center in the viewport                                                                                                                                   
   The risk score "scale" might be off? I'm seeing risk_score                                                                                                                                      
   0.009132420091324202 for example, and obviously that rounds to 0/100 in the "Risk Level" blurb above on the right side.  I don't think you need to list risk score below, we need to just       
   adjust the panel to have the right scale? Concern what the actual range of risk scores are and adjust? Also, the risk "bubble" doesn't need to print the file name since that is represented    
   futher below in file.  However, that can easily be truncated.  Hovering over should show the full path.  The same goes for fqn.                                                                 
   When I hover over volitaility score, I'd like to understand what the definition of "volatility" is in this context? Similarly, what is risk score?                                              
   If I double click a node, it should be the center of the graph and everything expanding from it.                                                                                                
   The color of the dots (nodes) in the graph should be different depending on the label type; domain,feature,class,function,file,etc... and there should be a legend at the bottom that shows     
   what the colors are.  