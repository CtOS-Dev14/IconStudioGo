export namespace peicon {
	
	export class IconLayerInfo {
	    width: number;
	    height: number;
	    raw_w: number;
	    raw_h: number;
	    colors: number;
	    planes: number;
	    bit_count: number;
	    size: number;
	    icon_id: number;
	    is_png: boolean;
	
	    static createFrom(source: any = {}) {
	        return new IconLayerInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.width = source["width"];
	        this.height = source["height"];
	        this.raw_w = source["raw_w"];
	        this.raw_h = source["raw_h"];
	        this.colors = source["colors"];
	        this.planes = source["planes"];
	        this.bit_count = source["bit_count"];
	        this.size = source["size"];
	        this.icon_id = source["icon_id"];
	        this.is_png = source["is_png"];
	    }
	}
	export class IconGroupSummary {
	    group_id: string;
	    total_layers: number;
	    has_uhd: boolean;
	    max_res: number;
	    max_bpp: number;
	    thumbnail: string;
	    layers: IconLayerInfo[];
	
	    static createFrom(source: any = {}) {
	        return new IconGroupSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.group_id = source["group_id"];
	        this.total_layers = source["total_layers"];
	        this.has_uhd = source["has_uhd"];
	        this.max_res = source["max_res"];
	        this.max_bpp = source["max_bpp"];
	        this.thumbnail = source["thumbnail"];
	        this.layers = this.convertValues(source["layers"], IconLayerInfo);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class ParseResult {
	    file_path: string;
	    file_name: string;
	    total_groups: number;
	    groups: IconGroupSummary[];
	
	    static createFrom(source: any = {}) {
	        return new ParseResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.file_path = source["file_path"];
	        this.file_name = source["file_name"];
	        this.total_groups = source["total_groups"];
	        this.groups = this.convertValues(source["groups"], IconGroupSummary);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace settings {
	
	export class AppSettings {
	    theme: string;
	    width: number;
	    height: number;
	    x: number;
	    y: number;
	    is_maximized: boolean;
	    has_position: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AppSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.theme = source["theme"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.x = source["x"];
	        this.y = source["y"];
	        this.is_maximized = source["is_maximized"];
	        this.has_position = source["has_position"];
	    }
	}

}

