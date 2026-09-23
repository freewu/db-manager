/**
 * The word lists the Chinese-friendly placeholders draw from.
 *
 * mock.js' own generators only know `@cname`, `@cfirst`, `@clast`, `@province`,
 * `@city`, `@county` and `@zip`, and even those ship their data inside the
 * library. This build implements them itself, so the data has to live
 * somewhere — here, as plain arrays, one concern per constant.
 *
 * Two things are deliberately flat. There is no province → city → county map:
 * a generated city need not sit in the generated province, because what these
 * rows are for is filling a table, not drawing a map. And the word lists are
 * samples, not dictionaries — they are long enough that a few hundred rows do
 * not repeat obviously, which is the whole requirement.
 */

/** The 100 most common surnames, plus a few two-character ones. */
export const SURNAMES = [
  '王', '李', '张', '刘', '陈', '杨', '黄', '赵', '周', '吴',
  '徐', '孙', '马', '朱', '胡', '郭', '何', '高', '林', '罗',
  '郑', '梁', '谢', '宋', '唐', '许', '韩', '冯', '邓', '曹',
  '彭', '曾', '肖', '田', '董', '袁', '潘', '于', '蒋', '蔡',
  '余', '杜', '叶', '程', '苏', '魏', '吕', '丁', '任', '沈',
  '姚', '卢', '姜', '崔', '钟', '谭', '陆', '汪', '范', '金',
  '石', '廖', '贾', '夏', '韦', '付', '方', '白', '邹', '孟',
  '熊', '秦', '邱', '江', '尹', '薛', '闫', '段', '雷', '侯',
  '龙', '史', '陶', '黎', '贺', '顾', '毛', '郝', '龚', '邵',
  '万', '钱', '严', '覃', '武', '戴', '莫', '孔', '向', '汤',
  '欧阳', '上官', '司马', '诸葛',
]

/** Characters a given name is spelled from. */
export const GIVEN_CHARS =
  '伟芳娜秀英敏静丽强磊军洋勇艳杰娟涛明超霞平刚桂建华文博子轩浩然思雨欣怡佳琪梓涵宇泽嘉诗梦瑶墨妍萱晨曦诚豪雯苑源婉婷俊熙艺书宜志晓东小红兰玉珍海燕丹婷莉倩园'

/** Characters used for the "random Chinese text" placeholders. */
export const CJK_CHARS =
  '的一是在不了有和人这中大为上个国我以要他时来用们生到作地于出就分对成会可主发年动同工也能下过子说产种面而方后多定行学法所民得经十三之进着等部度家电力里如水化高自二理起小物现实加量都两体制机当使点从业本去把性好应开它合还因由其些然前外天政四日那社义事平形相全表间样与关各重新线内数正心反你明看原又么利比或但质气第向道命此变条只没结解问意建月公无系军很情者最立代想已通并提直题党程展五果料象员革位入常文总次品式活设及管特件长求老头基资边流路级少图山统接知较将组见计别她手角期根论运农指几九区强放决西被干做必战先回则任取据处队南给色光门即保治北造百规热领七海口东导器压志世金增争济阶油思术极交受联什认六共权收证改清己美再采转更单风切打白教速花带安场身车例真务具万每目至达走积示议声报斗完类八离华名确才科张信马节话米整空元况今集温传土许步群广石记需段研界拉林律叫且究观越织装影算低持音众书布复容儿须际商非验连断深难近矿千周委素技备半办青省列习响约支般史感劳便团往酸历市克何除消构府称太准精值号率族维划选标写存候毛亲快效斯院查江型眼王按格养易置派层片始却专状育厂京识适属圆包火住调满县局照参红细引听该铁价严'

/** Province-level names, for `@province`. */
export const PROVINCES = [
  '北京市', '天津市', '河北省', '山西省', '内蒙古自治区', '辽宁省', '吉林省',
  '黑龙江省', '上海市', '江苏省', '浙江省', '安徽省', '福建省', '江西省',
  '山东省', '河南省', '湖北省', '湖南省', '广东省', '广西壮族自治区', '海南省',
  '重庆市', '四川省', '贵州省', '云南省', '西藏自治区', '陕西省', '甘肃省',
  '青海省', '宁夏回族自治区', '新疆维吾尔自治区', '台湾省', '香港特别行政区',
  '澳门特别行政区',
]

/** Prefecture-level names, for `@city`. */
export const CITIES = [
  '北京市', '上海市', '广州市', '深圳市', '杭州市', '南京市', '苏州市', '成都市',
  '武汉市', '西安市', '重庆市', '天津市', '长沙市', '郑州市', '青岛市', '宁波市',
  '东莞市', '佛山市', '合肥市', '福州市', '厦门市', '济南市', '沈阳市', '大连市',
  '哈尔滨市', '长春市', '石家庄市', '太原市', '南昌市', '昆明市', '贵阳市',
  '南宁市', '海口市', '兰州市', '银川市', '西宁市', '乌鲁木齐市', '呼和浩特市',
  '拉萨市', '无锡市', '常州市', '徐州市', '温州市', '嘉兴市', '绍兴市', '金华市',
  '泉州市', '珠海市', '中山市', '惠州市', '烟台市', '潍坊市', '洛阳市', '唐山市',
  '保定市', '南通市', '扬州市', '盐城市', '泰州市', '芜湖市',
]

/** District names, for `@county`. */
export const COUNTIES = [
  '朝阳区', '海淀区', '东城区', '西城区', '丰台区', '通州区', '浦东新区', '黄浦区',
  '徐汇区', '静安区', '长宁区', '普陀区', '天河区', '越秀区', '海珠区', '南山区',
  '福田区', '宝安区', '龙岗区', '西湖区', '拱墅区', '余杭区', '江宁区', '鼓楼区',
  '玄武区', '姑苏区', '吴中区', '武侯区', '锦江区', '青羊区', '江岸区', '武昌区',
  '洪山区', '雁塔区', '碑林区', '渝中区', '江北区', '和平区', '南开区', '岳麓区',
  '芙蓉区', '金水区', '二七区', '市南区', '历下区', '高新区', '经济开发区',
]

/** Street names, used by `@address`. */
export const STREETS = [
  '人民路', '解放路', '中山路', '建设路', '长江路', '黄河路', '文化路', '花园路',
  '科技路', '创业路', '光明路', '幸福路', '和平路', '新华路', '长安街', '南京路',
  '淮海路', '北京路', '香港路', '迎宾大道', '世纪大道', '东方大道', '滨江大道',
  '环城路', '学院路', '文昌路', '工业大道', '前进路', '青年路', '公园路',
  '民主路', '团结路', '胜利路', '复兴路', '富强路', '站前路', '泉城路', '观前街',
  '春熙路', '汉正街',
]

/** Company name fragments, used by `@company`. */
export const COMPANY_PREFIXES = [
  '华信', '中远', '恒达', '天翼', '盛世', '金石', '远景', '高科', '环宇', '汇通',
  '立信', '德胜', '广联', '兴源', '嘉华', '康泰', '众诚', '开元', '弘毅', '博远',
  '智邦', '优联', '泰和', '瑞丰', '卓越', '通达', '新元', '昌隆', '宝利', '双赢',
]

export const COMPANY_SUFFIXES = [
  '科技有限公司', '信息技术有限公司', '网络科技有限公司', '商贸有限公司',
  '实业有限公司', '电子有限公司', '咨询服务有限公司', '物流有限公司',
  '生物科技有限公司', '文化传媒有限公司',
]

/** What an `@email`/`@domain` local part is spelled from. */
export const LATIN_WORDS = [
  'alpha', 'amber', 'anchor', 'arbor', 'aspen', 'atlas', 'aurora', 'basil',
  'beacon', 'birch', 'bison', 'bloom', 'breeze', 'bronze', 'brook', 'cactus',
  'canvas', 'cedar', 'cinder', 'cobalt', 'comet', 'copper', 'coral', 'cosmos',
  'crane', 'crystal', 'dawn', 'delta', 'dune', 'ember', 'falcon', 'fern',
  'flint', 'forest', 'frost', 'garnet', 'glacier', 'granite', 'harbor', 'hazel',
  'heron', 'indigo', 'ivory', 'jasper', 'juniper', 'kelp', 'lantern', 'larch',
  'linen', 'lotus', 'lunar', 'maple', 'marble', 'meadow', 'mesa', 'meteor',
  'mica', 'mistral', 'nebula', 'nickel', 'nimbus', 'north', 'oak', 'oasis',
  'onyx', 'opal', 'orbit', 'otter', 'pebble', 'pine', 'plum', 'prairie', 'quartz',
  'quill', 'raven', 'reef', 'ridge', 'river', 'saffron', 'sage', 'silk', 'silver',
  'solstice', 'sparrow', 'spruce', 'stellar', 'summit', 'thistle', 'timber',
  'topaz', 'tundra', 'velvet', 'vertex', 'willow', 'winter', 'zenith', 'zephyr',
]

/** Top level domains, for `@tld` and `@domain`. */
export const TLDS = ['com', 'cn', 'net', 'org', 'io', 'dev', 'me', 'info', 'biz', 'co']

/** Protocols, for `@protocol` and `@url`. */
export const PROTOCOLS = ['https', 'http', 'ftp', 'ws', 'wss']

/** Area codes an `@id` is built from; real prefixes, not a full code table. */
export const ID_AREAS = [
  '110101', '110105', '120101', '130102', '140105', '210102', '220102', '230103',
  '310101', '310104', '320102', '330102', '340102', '350102', '360102', '370102',
  '410102', '420102', '430102', '440103', '440305', '450102', '500103', '510104',
  '520102', '530102', '610102', '620102', '630102', '640102', '650102',
]

/** Mobile number prefixes, for `@phone`. */
export const MOBILE_PREFIXES = [
  '130', '131', '132', '133', '135', '136', '137', '138', '139', '147',
  '150', '151', '152', '153', '155', '156', '157', '158', '159', '166',
  '170', '171', '172', '173', '175', '176', '177', '178', '180', '181',
  '182', '183', '184', '185', '186', '187', '188', '189', '191', '198', '199',
]

/** Issuer prefixes, for `@bankcard` — the check digit is computed, not listed. */
export const BANK_PREFIXES = [
  '622202', '621226', '622848', '621700', '622588', '622609', '622262', '621661',
  '622700', '621758', '622845', '621288',
]

/** First characters of a licence plate: the province's own abbreviation. */
export const PLATE_PROVINCES = [
  '京', '津', '冀', '晋', '蒙', '辽', '吉', '黑', '沪', '苏', '浙', '皖', '闽',
  '赣', '鲁', '豫', '鄂', '湘', '粤', '桂', '琼', '渝', '川', '贵', '云', '藏',
  '陕', '甘', '青', '宁', '新',
]

/**
 * User agents for `@ua`.
 *
 * Real strings, because a generated one is meant to look like traffic a server
 * would actually see.
 */
export const USER_AGENTS = [
  'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36',
  'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.6 Safari/605.1.15',
  'Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:133.0) Gecko/20100101 Firefox/133.0',
  'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36',
  'Mozilla/5.0 (iPhone; CPU iPhone OS 18_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.1 Mobile/15E148 Safari/604.1',
  'Mozilla/5.0 (Linux; Android 15; Pixel 9) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Mobile Safari/537.36',
  'Mozilla/5.0 (iPad; CPU OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1',
  'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36 Edg/131.0.0.0',
  'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36',
  'Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)',
  'curl/8.9.1',
  'PostmanRuntime/7.42.0',
]
