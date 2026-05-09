import { AddMaterial } from "./components/AddMaterial";
import { MaterialList } from "./components/MaterialList";
import { AskPanel } from "./components/AskPanel";
import { RequestHistory } from "./components/RequestHistory";

export default function App() {
  return (
    <div className="wrap">
      <section>
        <h1>Workspace</h1>
        <AddMaterial />
        <h2>Materials</h2>
        <MaterialList />
      </section>
      <section>
        <h1>Ask</h1>
        <AskPanel />
        <RequestHistory />
      </section>
    </div>
  );
}
