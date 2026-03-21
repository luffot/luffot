export namespace config {
	
	export class AIProviderConfig {
	    name: string;
	    provider: string;
	    api_key: string;
	    model: string;
	    base_url: string;
	    timeout_seconds: number;
	
	    static createFrom(source: any = {}) {
	        return new AIProviderConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.provider = source["provider"];
	        this.api_key = source["api_key"];
	        this.model = source["model"];
	        this.base_url = source["base_url"];
	        this.timeout_seconds = source["timeout_seconds"];
	    }
	}
	export class AIConfig {
	    enabled: boolean;
	    default_provider: string;
	    max_context_rounds: number;
	    timeout_seconds: number;
	    providers: AIProviderConfig[];
	
	    static createFrom(source: any = {}) {
	        return new AIConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.default_provider = source["default_provider"];
	        this.max_context_rounds = source["max_context_rounds"];
	        this.timeout_seconds = source["timeout_seconds"];
	        this.providers = this.convertValues(source["providers"], AIProviderConfig);
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
	
	export class AlertConfig {
	    enabled: boolean;
	    keywords: string[];
	    filter_keywords: string[];
	
	    static createFrom(source: any = {}) {
	        return new AlertConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.keywords = source["keywords"];
	        this.filter_keywords = source["filter_keywords"];
	    }
	}
	export class ProcessMonitorConfig {
	    use_vlmodel: boolean;
	    vlmodel_provider: string;
	    vlmodel_prompt: string;
	
	    static createFrom(source: any = {}) {
	        return new ProcessMonitorConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.use_vlmodel = source["use_vlmodel"];
	        this.vlmodel_provider = source["vlmodel_provider"];
	        this.vlmodel_prompt = source["vlmodel_prompt"];
	    }
	}
	export class DingTalkConfig {
	    source_mode: string;
	
	    static createFrom(source: any = {}) {
	        return new DingTalkConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source_mode = source["source_mode"];
	    }
	}
	export class SessionConfig {
	    source: string;
	    fixed_name: string;
	    script: string;
	
	    static createFrom(source: any = {}) {
	        return new SessionConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.fixed_name = source["fixed_name"];
	        this.script = source["script"];
	    }
	}
	export class ParseRules {
	    sender_pattern: string;
	    time_pattern: string;
	    content_mode: string;
	    dedup_enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ParseRules(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sender_pattern = source["sender_pattern"];
	        this.time_pattern = source["time_pattern"];
	        this.content_mode = source["content_mode"];
	        this.dedup_enabled = source["dedup_enabled"];
	    }
	}
	export class AppConfigItem {
	    name: string;
	    process_name: string;
	    display_name: string;
	    enabled: boolean;
	    icon_path: string;
	    parse_rules: ParseRules;
	    session_config: SessionConfig;
	    dingtalk: DingTalkConfig;
	    process_monitor: ProcessMonitorConfig;
	
	    static createFrom(source: any = {}) {
	        return new AppConfigItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.process_name = source["process_name"];
	        this.display_name = source["display_name"];
	        this.enabled = source["enabled"];
	        this.icon_path = source["icon_path"];
	        this.parse_rules = this.convertValues(source["parse_rules"], ParseRules);
	        this.session_config = this.convertValues(source["session_config"], SessionConfig);
	        this.dingtalk = this.convertValues(source["dingtalk"], DingTalkConfig);
	        this.process_monitor = this.convertValues(source["process_monitor"], ProcessMonitorConfig);
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
	export class BarrageHighlightRule {
	    keyword: string;
	    color: string;
	
	    static createFrom(source: any = {}) {
	        return new BarrageHighlightRule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.keyword = source["keyword"];
	        this.color = source["color"];
	    }
	}
	export class BarrageConfig {
	    filter_keywords: string[];
	    highlight_rules: BarrageHighlightRule[];
	
	    static createFrom(source: any = {}) {
	        return new BarrageConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.filter_keywords = source["filter_keywords"];
	        this.highlight_rules = this.convertValues(source["highlight_rules"], BarrageHighlightRule);
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
	
	export class CameraGuardConfig {
	    enabled: boolean;
	    interval_seconds: number;
	    provider_name: string;
	    confirm_count: number;
	    cooldown_seconds: number;
	
	    static createFrom(source: any = {}) {
	        return new CameraGuardConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.interval_seconds = source["interval_seconds"];
	        this.provider_name = source["provider_name"];
	        this.confirm_count = source["confirm_count"];
	        this.cooldown_seconds = source["cooldown_seconds"];
	    }
	}
	
	export class LangfuseConfig {
	    enabled: boolean;
	    public_key: string;
	    secret_key: string;
	    base_url: string;
	    async_enabled: boolean;
	    batch_size: number;
	    flush_interval: number;
	
	    static createFrom(source: any = {}) {
	        return new LangfuseConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.public_key = source["public_key"];
	        this.secret_key = source["secret_key"];
	        this.base_url = source["base_url"];
	        this.async_enabled = source["async_enabled"];
	        this.batch_size = source["batch_size"];
	        this.flush_interval = source["flush_interval"];
	    }
	}
	
	

}

export namespace scheduler {
	
	export class Scheduler {
	
	
	    static createFrom(source: any = {}) {
	        return new Scheduler(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}

}

