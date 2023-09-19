export namespace backend {
	
	
	
	
	
	
	
	
	
	
	export class HTMLMeta {
	    siteName: string;
	    title: string;
	    description: string;
	    image: string;
	
	    static createFrom(source: any = {}) {
	        return new HTMLMeta(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.siteName = source["siteName"];
	        this.title = source["title"];
	        this.description = source["description"];
	        this.image = source["image"];
	    }
	}
	

}

