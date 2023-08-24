import {useEffect, useState} from "react";
import {GetChains} from "../../wailsjs/go/main/App";
import MasterDetail from "./MasterDetail";
import {message} from "antd";

const Chains = () => {
    const [chains, setChains] = useState([]);
    const [messageApi, contextHolder] = message.useMessage();

    useEffect(() => {
        const getChains = async () => {
            GetChains()
                .then((chains) => {
                    setChains(chains);
                })
                .catch((error) => {
                    messageApi.open({
                        type: "error", content: error,
                    });
                });
        };
        getChains();
    }, []);

    const title = "Chains";
    const getItemDescription = (chain) => chain;
    const detailLayout = (chain) => (<ChainDetails chain={chain}/>);

    return (<>
            {contextHolder}
            <MasterDetail
                title={title}
                items={chains}
                getItemDescription={getItemDescription}
                detailLayout={detailLayout}
            />
        </>);
};

export default Chains;
